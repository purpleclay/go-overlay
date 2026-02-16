# Builds a Go tool from a tool manifest using the existing builder infrastructure.
# Each tool manifest contains the module path, version, NAR hash for the source,
# sub-packages to build, and a full dependency tree (mod section) compatible with mkVendorEnv.
{
  lib,
  stdenv,
  fetchGoModule,
  mkVendorEnv,
}: {
  manifest,
  go,
  pname,
}: let
  inherit (lib) concatStringsSep;

  # Fetch the tool's own source from the Go module proxy
  toolSrc = fetchGoModule {
    goPackagePath = manifest.module;
    version = "v${manifest.version}";
    hash = manifest.hash;
    inherit go;
  };

  # Create vendor directory from the manifest's mod section
  vendorEnv = mkVendorEnv {
    inherit go;
    manifest = {mod = manifest.mod;};
  };

  subPackages = manifest.subPackages or ["."];
in
  stdenv.mkDerivation {
    inherit pname;
    version = manifest.version;
    src = toolSrc;

    nativeBuildInputs = [go];

    inherit (go) GOOS GOARCH CGO_ENABLED;

    GO111MODULE = "on";
    GOFLAGS = "-mod=vendor";

    configurePhase = ''
      runHook preConfigure

      export GOCACHE=$TMPDIR/go-cache
      export GOPATH="$TMPDIR/go"
      export GOPROXY=off
      export GOSUMDB=off

      rm -rf vendor
      cp --no-preserve=mode -rs ${vendorEnv} vendor
      chmod -R u+w vendor

      runHook postConfigure
    '';

    buildPhase = ''
      runHook preBuild

      buildFlags=(
        -v
        -p $NIX_BUILD_CORES
      )

      for pkg in ${concatStringsSep " " subPackages}; do
        echo "Building $pkg"
        go install "''${buildFlags[@]}" "./$pkg"
      done

      runHook postBuild
    '';

    installPhase = ''
      runHook preInstall

      mkdir -p $out
      if [ -d "$GOPATH/bin" ]; then
        cp -r "$GOPATH/bin" $out/
      fi

      runHook postInstall
    '';

    passthru = {inherit go vendorEnv;};

    meta = {
      description = "${pname} - built from ${manifest.module}@v${manifest.version}";
      homepage = "https://pkg.go.dev/${manifest.module}";
      license = lib.licenses.bsd3;
    };
  }
