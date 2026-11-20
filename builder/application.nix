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
  mkTestPackages,
}: let
  inherit (builtins) fromTOML readFile;
  inherit (lib) optionalString pathExists;
in {
  # Build a single-module Go application from a govendor.toml manifest.
  # Defaults to src + "/govendor.toml" — run `govendor` to generate it.
  # Builds that share a manifest and go can share one mkVendorEnv result as vendorEnv.
  # For projects using `go mod vendor`, use buildGoVendoredApplication instead.
  buildGoApplication = {
    pname,
    version,
    src,
    modules ? src + "/govendor.toml", # Path to govendor.toml manifest
    go,
    vendorEnv ? null, # mkVendorEnv result for modules and go; assembled here when null
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

    # These options only shape the vendor environment. Ignoring one beside a
    # prebuilt environment would build something other than what was asked
    # for, so one set to anything but its default of {}, null or "" is refused.
    vendorOptions = {inherit localReplaces netrcFile GOPRIVATE GONOSUMDB GONOPROXY;};
    refusedOptions = builtins.attrNames (lib.filterAttrs (_: value: !builtins.elem value [{} null ""]) vendorOptions);

    vendor =
      if vendorEnv == null
      then mkVendorEnv ({inherit go manifest src;} // vendorOptions)
      else if refusedOptions == []
      then vendorEnv
      else throw "go-overlay: vendorEnv cannot be combined with ${lib.concatStringsSep ", " refusedOptions}; pass them to mkVendorEnv instead";

    configurePhase =
      attrs.configurePhase or ''
        runHook preConfigure

        export GOCACHE=$TMPDIR/go-cache
        export GOPATH="$TMPDIR/go"

        rm -rf vendor
        ${
          if vendor.useSymlinks
          then "cp --no-preserve=mode -rs ${vendor} vendor"
          else "cp -r --reflink=auto ${vendor} vendor"
        }
        chmod -R u+w vendor

        runHook postConfigure
      '';

    testPackages = mkTestPackages {
      listCmd = "go list ./...";
      basePackages = "./...";
      inherit excludedPackages;
    };

    hostTools = map (pkg:
      mkHostTool {
        inherit src go pkg;
        inherit (vendor) useSymlinks;
        vendorEnv = vendor;
        version = manifest.tool.${pkg}.version;
        GOWORK = "off";
      })
    (builtins.attrNames (manifest.tool or {}));

    passthru = {
      inherit go hostTools;
      vendorEnv = vendor;
    };
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

      testPackages = mkTestPackages {
        listCmd = "go list ./...";
        basePackages = "./...";
        inherit excludedPackages;
      };

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
