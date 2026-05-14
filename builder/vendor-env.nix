# Constructs a vendor directory with modules.txt from a govendor.toml manifest.
# For Go >= 1.25, modules are symlinked for efficiency (GODEBUG=embedfollowsymlinks=1
# handles //go:embed compatibility). For older Go, modules are copied using
# cp -r --reflink=auto to avoid //go:embed rejecting symlinks as irregular files.
{
  lib,
  runCommand,
  fetchGoModule,
}: let
  inherit (lib) concatMapStringsSep escapeShellArg optionalString;

  # Generate modules.txt entry for a single module
  mkModuleEntry = goPackagePath: meta: let
    isRemoteReplace = (meta ? replaced) && meta.replaced != goPackagePath;
    header =
      if meta ? local
      then "# ${goPackagePath} ${meta.version} => ${meta.local}"
      else if isRemoteReplace
      then "# ${goPackagePath} ${meta.version} => ${meta.replaced} ${meta.version}"
      else "# ${goPackagePath} ${meta.version}";
    explicit =
      if meta.go or "" != ""
      then "## explicit; go ${meta.go}"
      else "## explicit";
    packages = concatMapStringsSep "\n" (p: p) (meta.packages or []);
  in
    header + "\n" + explicit + optionalString (packages != "") ("\n" + packages);

  # Generate shell commands to copy fetched modules into $out directory.
  # Handles overlapping module paths by processing deepest paths first and
  # using symlinks where possible for performance.
  mkModuleCopyCommands = {
    sources,
    useSymlinks ? true,
  }: let
    pkgPaths = builtins.attrNames sources;
    # Lexicographic sort puts prefix paths before longer paths within the same domain.
    # Reversing ensures deeper paths (e.g. foo/bar/v2) are placed before their parent
    # (foo/bar), so the symlink for the parent is never created before its children
    # need to write into it.
    pkgPathsSortedByDepth = lib.lists.reverseList (lib.lists.sort (p: q: p < q) pkgPaths);
  in ''
    shopt -s dotglob

    ${concatMapStringsSep "\n" (
        goPackagePath: let
          modSrc = sources.${goPackagePath};
        in
          if useSymlinks
          then ''
            if [ -d "$out/${escapeShellArg goPackagePath}" ]; then
                cp -rs --update=none ${modSrc}/* "$out/${escapeShellArg goPackagePath}/"
            else
                mkdir -p "$out/$(dirname ${escapeShellArg goPackagePath})"
                ln -s ${modSrc} "$out/${escapeShellArg goPackagePath}"
            fi
          ''
          else ''
            if [ -d "$out/${escapeShellArg goPackagePath}" ]; then
                cp -r --reflink=auto --update=none ${modSrc}/* "$out/${escapeShellArg goPackagePath}/"
            else
                mkdir -p "$out/$(dirname ${escapeShellArg goPackagePath})"
                cp -r --reflink=auto ${modSrc} "$out/${escapeShellArg goPackagePath}"
            fi
          ''
      )
      pkgPathsSortedByDepth}

    shopt -u dotglob
  '';
in {
  inherit mkModuleCopyCommands;

  mkVendorEnv = {
    go,
    manifest, # Parsed govendor.toml (via builtins.fromTOML)
    src ? null, # Source tree for local module replacements
    localReplaces ? {}, # Map of module path to Nix path for external local replaces
    netrcFile ? null,
    GOPRIVATE ? "",
    GONOSUMDB ? "",
    GONOPROXY ? "",
  }: let
    useSymlinks = lib.versionAtLeast go.version "1.25";
    modules = manifest.mod or {};

    remoteModules = lib.filterAttrs (_: meta: !(meta ? local)) modules;
    localModules = lib.filterAttrs (_: meta: meta ? local) modules;

    # For remote path replacements (replace A => B version), govendor hashes the
    # replacement module B, so we must fetch B — not A — to match the stored hash.
    sources =
      builtins.mapAttrs (
        goPackagePath: meta:
          fetchGoModule {
            goPackagePath =
              if (meta ? replaced) && meta.replaced != goPackagePath
              then meta.replaced
              else goPackagePath;
            inherit go netrcFile GOPRIVATE GONOSUMDB GONOPROXY;
            inherit (meta) version hash;
          }
      )
      remoteModules;

    modulesTxt = let
      moduleEntries = concatMapStringsSep "\n" (
        goPackagePath: mkModuleEntry goPackagePath modules.${goPackagePath}
      ) (builtins.attrNames modules);

      localTrailers = concatMapStringsSep "\n" (
        goPackagePath: let
          meta = localModules.${goPackagePath};
        in "# ${goPackagePath} => ${meta.local}"
      ) (builtins.attrNames localModules);

      remoteReplaceModules = lib.filterAttrs (goPackagePath: meta: (meta ? replaced) && meta.replaced != goPackagePath) modules;
      remoteTrailers = concatMapStringsSep "\n" (
        goPackagePath: let
          meta = remoteReplaceModules.${goPackagePath};
        in "# ${goPackagePath} => ${meta.replaced} ${meta.version}"
      ) (builtins.attrNames remoteReplaceModules);
    in
      moduleEntries
      + optionalString (localTrailers != "") ("\n" + localTrailers)
      + optionalString (remoteTrailers != "") ("\n" + remoteTrailers);

    remoteCopyCommands = mkModuleCopyCommands {inherit sources useSymlinks;};

    localModuleSources =
      builtins.mapAttrs (
        goPackagePath: meta:
          if localReplaces ? ${goPackagePath}
          then localReplaces.${goPackagePath}
          else if src != null
          then "${src}/${meta.local}"
          else throw "go-overlay: Local module '${goPackagePath}' not found in localReplaces and no 'src' provided"
      )
      localModules;
  in
    runCommand "vendor-env"
    {
      passAsFile = ["modulesTxt"];
      inherit modulesTxt;
      passthru = {inherit sources useSymlinks;};
      # Add localReplaces paths as explicit derivation inputs so they're tracked
      # and fetched before the build runs (fixes CI builds where store paths
      # don't exist yet)
      localReplaceSrcs = lib.attrValues localReplaces;
    }
    ''
      mkdir -p $out

      # Copy remote modules
      ${remoteCopyCommands}

      # Copy local modules from source tree
      ${mkModuleCopyCommands {
        sources = localModuleSources;
        inherit useSymlinks;
      }}

      # Write modules.txt
      cp "$modulesTxtPath" "$out/modules.txt"
    '';
}
