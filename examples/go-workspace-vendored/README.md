# go-workspace-vendored

Builds the same mood check-in app as `go-workspace` using a committed `vendor/` directory instead of a `govendor.toml` manifest. This demonstrates the use of `buildGoVendoredWorkspace`.

## Getting started

Run the example:

```shell
nix run .#example-go-workspace-vendored
```

Then open [http://localhost:8080](http://localhost:8080) in your browser.

## The Nix bit

```nix
# No govendor.toml — buildGoVendoredWorkspace uses the committed vendor/ directory directly.
pkgs.buildGoVendoredWorkspace {
  inherit go;

  pname = "server";
  version = "0.1.0";
  src = ../go-workspace;
  subPackages = ["server"];
}
```

The `vendor/` directory is generated with `go work vendor` rather than `go mod vendor`, and is committed alongside the source.
