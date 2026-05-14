# hello-world

The simplest possible go-overlay build: a single binary with no external dependencies.

## Getting started

Run the example:

```shell
nix run .#example-hello-world
```

## The Nix bit

```nix
{
  pkgs,
  go,
}:
pkgs.buildGoApplication {
  inherit go;
  
  pname = "hello-world";
  version = "0.1.0";
  src = ./.;
}
```
