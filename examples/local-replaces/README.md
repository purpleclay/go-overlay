# local-replaces

A CLI, demonstrating `localReplaces` for building Go applications that depend on a local library module via a `replace` directive.

> [!IMPORTANT]
> This works locally but breaks inside the Nix sandbox, where relative paths do not exist. `localReplaces` bridges the gap by providing the Nix store path for each replaced module — the builder patches `vendor/modules.txt` so Go can resolve the library at build time.

## Getting started

```shell
nix run .#example-local-replaces
# Weather in London, United Kingdom
#   12.4°C  /  54.3°F
#
# Distance to the North Pole
#   5571 km  /  3462 mi
```

## The Nix bit

```nix
pkgs.buildGoApplication {
  inherit go;

  pname = "local-replaces";
  version = "0.1.0";
  src = ./.;
  modules = ./govendor.toml;

  # The go.mod replace directive points to ./units — a relative filesystem path
  # that does not exist inside the Nix sandbox. localReplaces maps each locally
  # replaced module to its Nix store path so the builder can wire it up correctly.
  localReplaces = {
    "github.com/go-overlay/examples/local-replaces/units" = ./units;
  };
}
```
