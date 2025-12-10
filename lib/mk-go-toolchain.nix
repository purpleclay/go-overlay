# Builds a Go toolchain derivation from manifest data
{
  lib,
  stdenv,
  fetchurl,
}: manifest: let
  platform = manifest.${stdenv.hostPlatform.system} or null;
in
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

      # Go binary distributions are pre-built and statically linked
      dontBuild = true;
      dontConfigure = true;
      dontStrip = true;
      dontPatchELF = true;
      dontFixup = true;

      installPhase = ''
        runHook preInstall
        mkdir -p $out
        cp -r ./* $out/
        runHook postInstall
      '';

      meta = {
        description = "The Go programming language";
        homepage = "https://go.dev/";
        license = lib.licenses.bsd3;
        maintainers = [];
        platforms = builtins.attrNames (lib.filterAttrs (n: v: builtins.isAttrs v) manifest);
      };
    }
