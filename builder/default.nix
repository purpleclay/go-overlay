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
  # For local modules (replace directives with local paths), the source is
  # provided separately and copied during the build phase.
  mkVendorEnv = {
    go,
    manifest, # Parsed govendor.toml (via builtins.fromTOML)
    src ? null, # Source tree for local module replacements
  }: let
    modules = manifest.mod or {};

    # Separate remote and local modules
    remoteModules = lib.filterAttrs (_: meta: !(meta ? local)) modules;
    localModules = lib.filterAttrs (_: meta: meta ? local) modules;

    # Fetch remote modules only
    sources =
      mapAttrs (
        goPackagePath: meta:
          fetchGoModule {
            inherit goPackagePath go;
            inherit (meta) version hash;
          }
      )
      remoteModules;

    # Generate the complete modules.txt content in Nix
    # Format for regular modules:
    # # module/path version
    # ## explicit; go X.Y
    # package/path1
    #
    # Format for local replacements (Go requires both lines):
    # # module/path version => ./local/path
    # ## explicit; go X.Y
    # package/path1
    # # module/path => ./local/path
    modulesTxt = let
      moduleEntries = concatMapStringsSep "\n" (
        goPackagePath: let
          meta = modules.${goPackagePath};
          header =
            if meta ? local
            then "# ${goPackagePath} ${meta.version} => ${meta.local}"
            else "# ${goPackagePath} ${meta.version}";
          explicit =
            if meta.go or "" != ""
            then "## explicit; go ${meta.go}"
            else "## explicit";
          packages = concatMapStringsSep "\n" (p: p) (meta.packages or []);
        in
          header + "\n" + explicit + optionalString (packages != "") ("\n" + packages)
      ) (builtins.attrNames modules);

      # Generate trailing replacement markers for local modules
      localTrailers = concatMapStringsSep "\n" (
        goPackagePath: let
          meta = localModules.${goPackagePath};
        in "# ${goPackagePath} => ${meta.local}"
      ) (builtins.attrNames localModules);
    in
      moduleEntries + optionalString (localTrailers != "") ("\n" + localTrailers);

    # Generate copy commands for remote modules
    # We use cp -r instead of symlinks to handle overlapping module paths
    # (e.g., go.opentelemetry.io/otel and go.opentelemetry.io/otel/trace)
    remoteCopyCommands = concatMapStringsSep "\n" (
      goPackagePath: let
        modSrc = sources.${goPackagePath};
      in ''
        mkdir -p "$out/${escapeShellArg goPackagePath}"
        cp -r ${modSrc}/* "$out/${escapeShellArg goPackagePath}/"
      ''
    ) (builtins.attrNames remoteModules);

    # Generate copy commands for local modules (from src)
    localCopyCommands =
      if src != null
      then
        concatMapStringsSep "\n" (
          goPackagePath: let
            meta = localModules.${goPackagePath};
            localPath = meta.local;
          in ''
            mkdir -p "$out/${escapeShellArg goPackagePath}"
            cp -r ${src}/${escapeShellArg localPath}/* "$out/${escapeShellArg goPackagePath}/"
          ''
        ) (builtins.attrNames localModules)
      else
        # If no src provided but there are local modules, error
        if localModules != {}
        then throw "go-overlay: Local modules found in manifest but no 'src' provided to mkVendorEnv"
        else "";
  in
    runCommand "vendor-env"
    {
      passAsFile = ["modulesTxt"];
      inherit modulesTxt;
      passthru = {inherit sources;};
    }
    ''
      mkdir -p $out

      # Copy remote modules
      ${remoteCopyCommands}

      # Copy local modules from source tree
      ${localCopyCommands}

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
    GOPROXY ? "off",
    GOPRIVATE ? "",
    GOSUMDB ? "off",
    GONOSUMDB ? "",
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
          inherit go manifest src;
        }
      else null;
  in
    # Validate: must have either modules or in-tree vendor
    if !useInTreeVendor && !useManifest
    then throw "go-overlay: No vendor source found. Provide 'modules' parameter pointing to govendor.toml, or include a vendor/ directory in src."
    else
      stdenv.mkDerivation (
        builtins.removeAttrs attrs ["modules" "subPackages" "ldflags" "tags" "GOPROXY" "GOPRIVATE" "GOSUMDB" "GONOSUMDB"]
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
                export GOPROXY=${escapeShellArg GOPROXY}
                export GOPRIVATE=${escapeShellArg GOPRIVATE}
                export GOSUMDB=${escapeShellArg GOSUMDB}
                export GONOSUMDB=${escapeShellArg GONOSUMDB}

                # Use in-tree vendor directory as-is
                chmod -R u+w vendor

                runHook postConfigure
              ''
              else ''
                runHook preConfigure

                export GOCACHE=$TMPDIR/go-cache
                export GOPATH="$TMPDIR/go"
                export GOPROXY=${escapeShellArg GOPROXY}
                export GOPRIVATE=${escapeShellArg GOPRIVATE}
                export GOSUMDB=${escapeShellArg GOSUMDB}
                export GONOSUMDB=${escapeShellArg GONOSUMDB}

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
