# Single-module Go application builders.
# buildGoApplication   — requires a govendor.toml manifest
# buildGoVendoredApplication — uses a committed vendor/ directory (go mod vendor)
{
  lib,
  stdenv,
  mkVendorEnv,
  mkHostTool,
  commonRemovedAttrs,
  mkCommonAttrs,
}: let
  inherit (builtins) fromTOML readFile;
  inherit (lib) concatMapStringsSep escapeShellArg optionalString pathExists;
in {
  # Build a single-module Go application from a govendor.toml manifest.
  # Defaults to src + "/govendor.toml" — run `govendor` to generate it.
  # For projects using `go mod vendor`, use buildGoVendoredApplication instead.
  buildGoApplication = {
    pname,
    version,
    src,
    modules ? src + "/govendor.toml", # Path to govendor.toml manifest
    go,
    subPackages ? ["."],
    ldflags ? [],
    tags ? [],
    allowGoReference ? false,
    localReplaces ? {},
    netrcFile ? null,
    GOPRIVATE ? "",
    GONOSUMDB ? "",
    GONOPROXY ? "",
    checkFlags ? [],
    extraGoFlags ? [],
    excludedPackages ? [],
    CGO_ENABLED ? go.CGO_ENABLED,
    GOOS ? go.GOOS,
    GOARCH ? go.GOARCH,
    ...
  } @ attrs: let
    manifest =
      if pathExists modules
      then fromTOML (readFile modules)
      else
        throw ''
          buildGoApplication: govendor.toml not found at ${toString modules}

            Generate one by running:
              govendor

            Or specify a custom path:
              buildGoApplication {
                modules = ./path/to/govendor.toml;
              }
        '';

    vendorEnv = mkVendorEnv {
      inherit go manifest src localReplaces netrcFile GOPRIVATE GONOSUMDB GONOPROXY;
    };

    configurePhase =
      attrs.configurePhase or ''
        runHook preConfigure

        export GOCACHE=$TMPDIR/go-cache
        export GOPATH="$TMPDIR/go"

        rm -rf vendor
        ${
          if vendorEnv.useSymlinks
          then "cp --no-preserve=mode -rs ${vendorEnv} vendor"
          else "cp -r --reflink=auto ${vendorEnv} vendor"
        }
        chmod -R u+w vendor

        runHook postConfigure
      '';

    testPackages =
      if excludedPackages == []
      then "./..."
      else "$(go list ./... | grep -F -v -- ${concatMapStringsSep " | grep -F -v -- " (p: escapeShellArg p) excludedPackages} || echo './...')";

    hostTools = map (pkg:
      mkHostTool {
        inherit src go pkg;
        inherit (vendorEnv) useSymlinks;
        vendorEnv = vendorEnv;
        version = manifest.tool.${pkg}.version;
        GOWORK = "off";
      })
    (builtins.attrNames (manifest.tool or {}));

    passthru = {inherit go vendorEnv;};
  in
    stdenv.mkDerivation (
      builtins.removeAttrs attrs commonRemovedAttrs
      // {inherit pname version src;}
      // mkCommonAttrs {
        inherit attrs go allowGoReference ldflags tags GOOS GOARCH CGO_ENABLED hostTools;
        inherit subPackages checkFlags extraGoFlags testPackages configurePhase passthru;
        useVendor = true;
        GOWORK = "off";
      }
    );

  # Build a single-module Go application using an in-tree vendor/ directory
  # committed via `go mod vendor`. No govendor.toml is required.
  # Unlike buildGoApplication, this builder does not provide drift detection,
  # per-dependency hash verification, or Go module tool directive injection.
  buildGoVendoredApplication = {
    pname,
    version,
    src,
    go,
    subPackages ? ["."],
    ldflags ? [],
    tags ? [],
    allowGoReference ? false,
    checkFlags ? [],
    extraGoFlags ? [],
    excludedPackages ? [],
    CGO_ENABLED ? go.CGO_ENABLED,
    GOOS ? go.GOOS,
    GOARCH ? go.GOARCH,
    ...
  } @ attrs:
    if !pathExists (src + "/vendor")
    then
      throw ''
        buildGoVendoredApplication: no vendor/ directory found in src.

          Commit a vendor directory by running:
            go mod vendor
      ''
    else let
      configurePhase =
        attrs.configurePhase or ''
          runHook preConfigure

          export GOCACHE=$TMPDIR/go-cache
          export GOPATH="$TMPDIR/go"

          chmod -R u+w vendor

          runHook postConfigure
        '';

      testPackages =
        if excludedPackages == []
        then "./..."
        else "$(go list ./... | grep -F -v -- ${concatMapStringsSep " | grep -F -v -- " (p: escapeShellArg p) excludedPackages} || echo './...')";

      passthru = {inherit go;};
    in
      stdenv.mkDerivation (
        builtins.removeAttrs attrs commonRemovedAttrs
        // {inherit pname version src;}
        // mkCommonAttrs {
          inherit attrs go allowGoReference ldflags tags GOOS GOARCH CGO_ENABLED;
          inherit subPackages checkFlags extraGoFlags testPackages configurePhase passthru;
          useVendor = true;
          hostTools = [];
          GOWORK = "off";
        }
      );
}
