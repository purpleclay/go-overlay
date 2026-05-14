# oapi-codegen

An HTTP server whose boilerplate is generated from an OpenAPI spec using [oapi-codegen](https://github.com/oapi-codegen/oapi-codegen), demonstrating govendor's native support for the Go `tool` directive.

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

  pname = "oapi-codegen";
  version = "0.1.0";
  src = ./.;

  # oapi-codegen is declared as a tool directive in go.mod. govendor compiles
  # it for the host platform and injects the binary into nativeBuildInputs,
  # making it available in $PATH here without any extra configuration.
  preBuild = ''
    oapi-codegen --config=api/oapi-codegen.yaml api/catto.yaml
  '';
}
```

The tool is invoked by its binary name directly — no `go generate`, no `go tool`. Because govendor compiles the tool for the **host** platform and injects it into `nativeBuildInputs`, this works correctly under cross-compilation without any workarounds.
