# Builds a Go toolchain derivation from manifest data
{
  lib,
  stdenv,
  fetchurl,
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

        passthru = lib.optionalAttrs (mkToolSet != null) {
          tools = mkToolSet self;
        };

        meta = {
          description = "The Go programming language";
          homepage = "https://go.dev/";
          license = lib.licenses.bsd3;
          maintainers = [];
          platforms = builtins.attrNames (lib.filterAttrs (n: v: builtins.isAttrs v) manifest);
        };
      };
in
  self
