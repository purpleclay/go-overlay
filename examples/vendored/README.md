# vendored

A lottery number generator styled with [lipgloss](https://github.com/charmbracelet/lipgloss), demonstrating `buildGoVendoredApplication` with a committed `vendor/` directory.

## Getting started

Run the example:

```shell
nix run .#example-vendored
```

## The Nix bit

```nix
{
  pkgs,
  go,
}:
# No modules parameter — buildGoVendoredApplication detects the committed vendor/
# directory automatically and uses it in place of a govendor.toml manifest.
pkgs.buildGoVendoredApplication {
  inherit go;

  pname = "vendored";
  version = "0.1.0";
  src = ./.;
}
```

The `vendor/` directory is generated with `go mod vendor` and committed alongside the source.
