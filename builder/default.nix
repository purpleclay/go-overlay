# Builder for Go applications using go-overlay's govendor.toml manifest
# Unlike gomod2nix, this generates a vendor/modules.txt file so it works
# with unpatched Go toolchains from the official binary distributions.
{
  lib,
  stdenv,
  stdenvNoCC,
  runCommand,
  cacert,
  git,
  jq,
}: let
  inherit
    (builtins)
    fromTOML
    mapAttrs
    readFile
    ;

  inherit
    (lib)
    concatMapStringsSep
    concatStringsSep
    escapeShellArg
    optionalString
    ;

  # Fetch a Go module using `go mod download`.
  # Supports private modules via GOPRIVATE, GOPROXY, and .netrc.
  fetchGoModule = {
    goPackagePath,
    version,
    hash, # NAR hash from govendor.toml
    go,
  }:
    stdenvNoCC.mkDerivation {
      name = "${baseNameOf goPackagePath}_${version}";
      builder = ./fetch.sh;
      inherit goPackagePath version;
      nativeBuildInputs = [
        cacert
        git
        go
        jq
      ];
      outputHashMode = "recursive";
      outputHashAlgo = null;
      outputHash = hash;
      impureEnvVars = [
        "GOPROXY"
        "http_proxy"
        "https_proxy"
      ];
    };

  # Create a vendor directory with modules.txt from a govendor.toml manifest.
  # The vendor directory contains symlinks to fetched modules.
  mkVendorEnv = {
    go,
    manifest, # Parsed govendor.toml (via builtins.fromTOML)
  }: let
    modules = manifest.mod or {};

    # Fetch all modules
    sources =
      mapAttrs (
        goPackagePath: meta:
          fetchGoModule {
            inherit goPackagePath go;
            inherit (meta) version hash;
          }
      )
      modules;

    # Generate the complete modules.txt content in Nix
    # Format:
    # # module/path version
    # ## explicit; go X.Y
    # package/path1
    # package/path2
    modulesTxt = concatMapStringsSep "\n" (
      goPackagePath: let
        meta = modules.${goPackagePath};
        header = "# ${goPackagePath} ${meta.version}";
        explicit =
          if meta.go or "" != ""
          then "## explicit; go ${meta.go}"
          else "## explicit";
        packages = concatMapStringsSep "\n" (p: p) (meta.packages or []);
      in
        header + "\n" + explicit + optionalString (packages != "") ("\n" + packages)
    ) (builtins.attrNames modules);

    # Generate symlink commands in Nix to avoid jq in the build
    symlinkCommands = concatMapStringsSep "\n" (
      goPackagePath: let
        src = sources.${goPackagePath};
      in ''
        mkdir -p "$out/$(dirname ${escapeShellArg goPackagePath})"
        ln -s ${src} "$out/${escapeShellArg goPackagePath}"
      ''
    ) (builtins.attrNames modules);
  in
    runCommand "vendor-env"
    {
      passAsFile = ["modulesTxt"];
      inherit modulesTxt;
      passthru = {inherit sources;};
    }
    ''
      mkdir -p $out

      # Create symlinks for each module
      ${symlinkCommands}

      # Write modules.txt
      cp "$modulesTxtPath" "$out/modules.txt"
    '';

  # Build a Go application using vendored dependencies from govendor.toml.
  # Unlike buildGoModule, this works with unpatched Go from binary distributions.
  buildGoApplication = {
    pname,
    version,
    src,
    modules ? src + "/govendor.toml", # Path to govendor.toml manifest
    go, # Go derivation from go-overlay (e.g., go-bin.fromGoMod)
    subPackages ? ["."], # Packages to build (relative to src)
    ldflags ? [],
    tags ? [],
    CGO_ENABLED ? go.CGO_ENABLED,
    ...
  } @ attrs: let
    manifest = fromTOML (readFile modules);

    vendorEnv = mkVendorEnv {
      inherit go manifest;
    };
  in
    stdenv.mkDerivation (
      builtins.removeAttrs attrs ["modules" "subPackages" "ldflags" "tags"]
      // {
        inherit pname version src;

        nativeBuildInputs =
          (attrs.nativeBuildInputs or [])
          ++ [
            go
          ];

        inherit (go) GOOS GOARCH;
        inherit CGO_ENABLED;

        GO111MODULE = "on";
        GOFLAGS = "-mod=vendor";

        configurePhase =
          attrs.configurePhase or ''
            runHook preConfigure

            export GOCACHE=$TMPDIR/go-cache
            export GOPATH="$TMPDIR/go"
            export GOSUMDB=off
            export GOPROXY=off

            # Copy vendor environment (dereference symlinks)
            rm -rf vendor
            cp -rL ${vendorEnv} vendor
            chmod -R u+w vendor

            runHook postConfigure
          '';

        buildPhase =
          attrs.buildPhase or ''
            runHook preBuild

            buildFlags=(
              -v
              -p $NIX_BUILD_CORES
              ${optionalString (tags != []) "-tags=${concatStringsSep "," tags}"}
              ${optionalString (ldflags != []) "-ldflags=${escapeShellArg (concatStringsSep " " ldflags)}"}
            )

            for pkg in ${concatStringsSep " " subPackages}; do
              echo "Building $pkg"
              go install "''${buildFlags[@]}" "./$pkg"
            done

            runHook postBuild
          '';

        installPhase =
          attrs.installPhase or ''
            runHook preInstall

            mkdir -p $out
            if [ -d "$GOPATH/bin" ]; then
              cp -r "$GOPATH/bin" $out/
            fi

            runHook postInstall
          '';

        passthru = {
          inherit go vendorEnv;
        };
      }
    );
in {
  inherit buildGoApplication mkVendorEnv fetchGoModule;
}
