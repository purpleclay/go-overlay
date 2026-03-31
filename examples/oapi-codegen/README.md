# oapi-codegen

An HTTP server whose boilerplate is generated from an OpenAPI spec using [oapi-codegen](https://github.com/oapi-codegen/oapi-codegen), demonstrating govendor support for the Go `tool` directive.

## Getting started

Run the example:

```shell
nix run .#example-oapi-codegen
```

Then open [http://localhost:8080](http://localhost:8080) in your browser.

## The Nix bit

```nix
pkgs.buildGoApplication {
  inherit go;

  pname = "catto";
  version = "0.1.0";
  src = ./.;
  modules = ./govendor.toml;

  # oapi-codegen is declared as a tool directive in go.mod rather than a
  # nativeBuildInput. govendor includes tool dependencies in govendor.toml
  # so the Go toolchain can find and run it from the vendor directory.
  preBuild = ''
    go generate ./...
  '';
}
```
