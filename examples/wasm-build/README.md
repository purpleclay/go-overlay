# wasm-build

A browser-based word counter compiled to WebAssembly, demonstrating how to use `mkVendorEnv` directly with `stdenv.mkDerivation` when `buildGoApplication` is not the right tool.

## Getting started

Build the example:

```shell
nix build .#example-wasm-build
```

Then serve the output with Docker. The Nix store is not shared with Docker by default, so copy the build output first:

```shell
cp -r result/ /tmp/wasm-build
docker run --rm -p 8080:80 -v /tmp/wasm-build:/usr/share/nginx/html:ro nginx:alpine
```

Then open [http://localhost:8080](http://localhost:8080) in your browser.

## The Nix bit

```nix
{
  pkgs,
  go,
}: let
  # mkVendorEnv is the low-level primitive that buildGoApplication builds on top
  # of. It fetches all remote dependencies and assembles a vendor directory so
  # the build can run fully offline inside the Nix sandbox.
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
      export GOCACHE=$TMPDIR/go-cache
      export GOPATH=$TMPDIR/go
      export GOPROXY=off

      # Wire in the vendored dependencies produced by mkVendorEnv
      cp --no-preserve=mode -rs ${vendorEnv} vendor
      chmod -R u+w vendor
    '';

    buildPhase = ''
      # buildGoApplication always targets the host platform and installs binaries
      # to $out/bin. A custom derivation is required here because the output is a
      # .wasm file, not a native executable, and the install layout is entirely
      # different.
      go build -trimpath -o wordcount.wasm .
    '';

    installPhase = ''
      mkdir -p $out

      install -m644 wordcount.wasm $out/wordcount.wasm
      install -m644 index.html $out/index.html

      # wasm_exec.js is the JS runtime bridge shipped with the Go toolchain.
      # It bootstraps the WASM module and wires up the Go<->JS interop layer.
      install -m644 "$(go env GOROOT)/lib/wasm/wasm_exec.js" $out/wasm_exec.js
    '';
  }
```

## Updating dependencies

Because `main.go` uses the `//go:build js && wasm` constraint, `govendor` must be told to resolve packages for the WebAssembly target explicitly:

```shell
govendor --include-platform js/wasm
```
