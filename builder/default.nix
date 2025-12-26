# Builder for Go applications using vendored dependencies.
# Supports two modes:
# 1. In-tree vendor: Use existing vendor/ directory from src
# 2. Manifest mode: Generate vendor from govendor.toml
#
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
    pathExists
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

    # Generate copy commands for each module
    # We use cp -r instead of symlinks to handle overlapping module paths
    # (e.g., go.opentelemetry.io/otel and go.opentelemetry.io/otel/trace)
    copyCommands = concatMapStringsSep "\n" (
      goPackagePath: let
        src = sources.${goPackagePath};
      in ''
        mkdir -p "$out/${escapeShellArg goPackagePath}"
        cp -r ${src}/* "$out/${escapeShellArg goPackagePath}/"
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

      # Copy each module
      ${copyCommands}

      # Write modules.txt
      cp "$modulesTxtPath" "$out/modules.txt"
    '';

  # Build a Go application using vendored dependencies.
  # Supports two modes:
  # 1. In-tree vendor: If modules is null and src contains vendor/, use it directly
  # 2. Manifest mode: Generate vendor from govendor.toml (modules parameter)
  #
  # Unlike buildGoModule, this works with unpatched Go from binary distributions.
  buildGoApplication = {
    pname,
    version,
    src,
    modules ? null, # Path to govendor.toml manifest (null = auto-detect)
    go, # Go derivation from go-overlay (e.g., go-bin.fromGoMod)
    subPackages ? ["."], # Packages to build (relative to src)
    ldflags ? [],
    tags ? [],
    CGO_ENABLED ? go.CGO_ENABLED,
    ...
  } @ attrs: let
    # Check for in-tree vendor directory
    hasInTreeVendor = pathExists (src + "/vendor");

    # Determine vendor mode
    useInTreeVendor = modules == null && hasInTreeVendor;
    useManifest = modules != null;

    # Only parse manifest and create vendorEnv when using manifest mode
    manifest =
      if useManifest
      then fromTOML (readFile modules)
      else null;

    vendorEnv =
      if useManifest
      then
        mkVendorEnv {
          inherit go manifest;
        }
      else null;
  in
    # Validate: must have either modules or in-tree vendor
    if !useInTreeVendor && !useManifest
    then throw "go-overlay: No vendor source found. Provide 'modules' parameter pointing to govendor.toml, or include a vendor/ directory in src."
    else
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
            attrs.configurePhase
            or (
              if useInTreeVendor
              then ''
                runHook preConfigure

                export GOCACHE=$TMPDIR/go-cache
                export GOPATH="$TMPDIR/go"
                export GOSUMDB=off
                export GOPROXY=off

                # Use in-tree vendor directory as-is
                chmod -R u+w vendor

                runHook postConfigure
              ''
              else ''
                runHook preConfigure

                export GOCACHE=$TMPDIR/go-cache
                export GOPATH="$TMPDIR/go"
                export GOSUMDB=off
                export GOPROXY=off

                # Copy vendor environment from manifest (dereference symlinks)
                rm -rf vendor
                cp -rL ${vendorEnv} vendor
                chmod -R u+w vendor

                runHook postConfigure
              ''
            );

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

          passthru =
            {inherit go;}
            // (
              if vendorEnv != null
              then {inherit vendorEnv;}
              else {}
            );
        }
      );
in {
  inherit buildGoApplication mkVendorEnv fetchGoModule;
}
