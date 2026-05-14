# Builds a single Go tool package for the host platform from an existing vendorEnv.
# Used to populate nativeBuildInputs so tool binaries are available in $PATH during
# preBuild without any user configuration.
#
# Also exports parseGoWorkModules, used by workspace builders to derive test targets
# from go.work when no manifest is available.
{
  lib,
  stdenv,
}: let
  inherit (lib) escapeShellArg optionalString splitString;
in {
  # Parse workspace member paths from a go.work file's content.
  # Handles both single-line (use ./path) and block (use (\n  ./path\n)) forms.
  # Returns a list of path strings, e.g. ["./mood" "./server"].
  parseGoWorkModules = content: let
    lines = splitString "\n" content;
    result =
      builtins.foldl' (
        acc: line: let
          startBlock = builtins.match "[ \t]*use[ \t]+\\([ \t]*" line;
          endBlock = builtins.match "[ \t]*\\)[ \t]*" line;
          singleUse = builtins.match "[ \t]*use[ \t]+([^ \t]+)[ \t]*" line;
          blockEntry = builtins.match "[ \t]*([^ \t/][^ \t]*)[ \t]*" line;
        in
          if acc.inBlock
          then
            if endBlock != null
            then acc // {inBlock = false;}
            else if blockEntry != null
            then acc // {mods = acc.mods ++ [(builtins.head blockEntry)];}
            else acc
          else if startBlock != null
          then acc // {inBlock = true;}
          else if singleUse != null
          then acc // {mods = acc.mods ++ [(builtins.head singleUse)];}
          else acc
      ) {
        inBlock = false;
        mods = [];
      }
      lines;
  in
    result.mods;

  mkHostTool = {
    version,
    src,
    go,
    vendorEnv,
    pkg, # the tool package path, e.g. "github.com/a-h/templ/cmd/templ"
    useSymlinks,
    GOWORK ? null, # "off" for buildGoApplication; null for workspace
    goWorkContent ? null, # generated go.work content for manifest-only workspaces
  }:
    stdenv.mkDerivation {
      pname = baseNameOf pkg;
      inherit version src;

      nativeBuildInputs = [go];

      env =
        {
          GOFLAGS = "-mod=vendor";
          GO111MODULE = "on";
          GOTOOLCHAIN = "local";
          GOPROXY = "off";
          # Build for the host platform — no GOOS/GOARCH override.
          CGO_ENABLED = go.CGO_ENABLED;
          GODEBUG = lib.optionalString (lib.versionAtLeast go.version "1.25") "embedfollowsymlinks=1";
        }
        // lib.optionalAttrs (GOWORK != null) {inherit GOWORK;};

      configurePhase = ''
        runHook preConfigure
        export GOCACHE=$TMPDIR/go-cache
        export GOPATH="$TMPDIR/go"
        ${optionalString (goWorkContent != null) ''
          if [ ! -f go.work ]; then
            printf '%s' ${escapeShellArg goWorkContent} > go.work
          fi
        ''}
        rm -rf vendor
        ${
          if useSymlinks
          then "cp --no-preserve=mode -rs ${vendorEnv} vendor"
          else "cp -r --reflink=auto ${vendorEnv} vendor"
        }
        chmod -R u+w vendor
        runHook postConfigure
      '';

      buildPhase = ''
        runHook preBuild
        go install -v -p $NIX_BUILD_CORES "${pkg}"
        runHook postBuild
      '';

      installPhase = ''
        runHook preInstall
        mkdir -p $out
        cp -r "$GOPATH/bin" $out/
        runHook postInstall
      '';

      strictDeps = true;
    };
}
