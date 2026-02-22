# Builds a Go toolchain derivation from manifest data
{
  lib,
  stdenv,
  fetchurl,
  symlinkJoin,
  mkToolSet ? null,
}: manifest: let
  platform = manifest.${stdenv.hostPlatform.system} or null;

  self =
    if platform == null
    then throw "go-overlay: Go ${manifest.version} is not available for ${stdenv.hostPlatform.system}"
    else
      stdenv.mkDerivation {
        pname = "go";
        version = manifest.version;

        src = fetchurl {
          url = platform.url;
          sha256 = platform.sha256;
        };

        # Expose GOOS, GOARCH, and CGO_ENABLED for compatibility with buildGoModule
        inherit (stdenv.targetPlatform.go) GOOS GOARCH;
        CGO_ENABLED =
          if stdenv.targetPlatform.isWasi || (stdenv.targetPlatform.isPower64 && stdenv.targetPlatform.isBigEndian)
          then 0
          else 1;

        # Go binary distributions are pre-built and statically linked
        dontBuild = true;
        dontConfigure = true;
        dontStrip = true;
        dontPatchELF = true;
        dontFixup = true;

        installPhase = ''
          runHook preInstall

          # Install Go distribution to share/go (matching nixpkgs structure)
          mkdir -p $out/share/go
          cp -r ./* $out/share/go/

          # Create bin directory with symlinks to the Go binaries
          mkdir -p $out/bin
          ln -s $out/share/go/bin/go $out/bin/go
          ln -s $out/share/go/bin/gofmt $out/bin/gofmt

          runHook postInstall
        '';

        # tools: attribute set of all bundled tools, keyed by name. Each tool
        #   exposes versioned attributes and a `latest` convenience attribute
        #   that resolves to the newest compatible version for this toolchain.
        #   Example: go.tools.govulncheck.latest, go.tools.delve."1.24.2"
        #
        # withTools: convenience function that bundles this Go toolchain with
        #   selected tools into a single derivation using symlinkJoin. Accepts
        #   a list of tool names and resolves each to its latest compatible version.
        #   Example: go.withTools ["govulncheck" "golangci-lint" "delve"]
        passthru = lib.optionalAttrs (mkToolSet != null) (let
          toolSet = mkToolSet self;
        in {
          tools = toolSet;
          withTools = toolNames: let
            availableTools = builtins.attrNames toolSet;
            resolvedTools =
              map (
                name:
                  if toolSet ? ${name}
                  then toolSet.${name}.latest
                  else throw "go-overlay: unknown tool '${name}'. Available tools: ${lib.concatStringsSep ", " availableTools}"
              )
              toolNames;
          in
            symlinkJoin {
              name = "go-${manifest.version}-with-tools";
              paths = [self] ++ resolvedTools;
            };
        });

        meta = {
          description = "The Go programming language";
          homepage = "https://go.dev/";
          license = lib.licenses.bsd3;
          maintainers = [];
          platforms = builtins.attrNames (lib.filterAttrs (n: v: builtins.isAttrs v) manifest);
          mainProgram = "go";
        };
      };
in
  self
