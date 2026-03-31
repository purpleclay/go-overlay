# nixpkgs-build-go-module

Builds the `hello-world` example using nixpkgs' `buildGoModule` with go-overlay's Go toolchain substituted in via `.override`.

## Getting started

```shell
nix run .#example-nixpkgs-build-go-module
# Hello, World!
```

## The Nix bit

```nix
{
  pkgs,
  go,
}:
# buildGoModule is nixpkgs' standard Go builder. Overriding its go attribute
# swaps in go-overlay's toolchain while keeping everything else from nixpkgs.
# Use this when you need nixpkgs ecosystem integration (e.g. NixOS modules,
# Home Manager packages) but still want go-overlay's Go version management.
(pkgs.buildGoModule.override {inherit go;}) {
  pname = "hello-world";
  version = "0.1.0";
  src = ../hello-world;
  vendorHash = null;
}
```
