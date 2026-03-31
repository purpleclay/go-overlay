{
  pkgs,
  go,
}: let
  # mkVendorEnv is the low-level primitive that buildGoApplication builds on top of.
  # It fetches all remote dependencies from the module proxy and assembles a vendor
  # directory, allowing the build to run fully offline inside the Nix sandbox.
  vendorEnv = pkgs.mkVendorEnv {
    inherit go;
    manifest = builtins.fromTOML (builtins.readFile ./govendor.toml);
  };
in
  pkgs.stdenv.mkDerivation {
    pname = "wordcount";
    version = "0.1.0";
    src = ./.;

    nativeBuildInputs = [go];

    # GOOS and GOARCH are set here rather than on the go build invocation so that
    # every go command in every phase targets the same platform automatically.
    env = {
      GOOS = "js";
      GOARCH = "wasm";
      CGO_ENABLED = "0";
      GO111MODULE = "on";
      GOFLAGS = "-mod=vendor";
    };

    configurePhase = ''
      runHook preConfigure

      export GOCACHE=$TMPDIR/go-cache
      export GOPATH=$TMPDIR/go
      export GOPROXY=off

      # Wire in the vendored dependencies produced by mkVendorEnv
      ${
        if vendorEnv.useSymlinks
        then "cp --no-preserve=mode -rs ${vendorEnv} vendor"
        else "cp -r --reflink=auto ${vendorEnv} vendor"
      }
      chmod -R u+w vendor

      runHook postConfigure
    '';

    buildPhase = ''
      runHook preBuild

      # buildGoApplication always targets the host platform and installs binaries
      # to $out/bin. A custom derivation is required here because the output is a
      # .wasm file, not a native executable, and the install layout is entirely
      # different.
      go build -trimpath -o wordcount.wasm .

      runHook postBuild
    '';

    installPhase = ''
      runHook preInstall

      mkdir -p $out

      install -m644 wordcount.wasm $out/wordcount.wasm
      install -m644 index.html $out/index.html

      # wasm_exec.js is the JS runtime bridge shipped with the Go toolchain.
      # It bootstraps the WASM module and wires up the Go<->JS interop layer.
      install -m644 "$(go env GOROOT)/lib/wasm/wasm_exec.js" $out/wasm_exec.js

      runHook postInstall
    '';
  }
