# http-chi-server

A minimal HTTP server built with [Chi](https://github.com/go-chi/chi), demonstrating how to manage external dependencies using a `govendor.toml` manifest.

## Getting started

Run the example:

```shell
nix run .#example-http-chi-server
```

Then query the server for an inspirational project name:

```shell
curl http://localhost:8080/name
# {"name":"caffeinated-goblin"}
```

## The Nix bit

```nix
pkgs.buildGoApplication {
  inherit go;

  pname = "http-chi-server";
  version = "0.1.0";
  src = ./.;
}
```
