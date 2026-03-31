# vendored

A lottery number generator styled with [lipgloss](https://github.com/charmbracelet/lipgloss), demonstrating `buildGoApplication` with a committed `vendor/` directory.

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
# No modules parameter — buildGoApplication detects the committed vendor/
# directory automatically and uses it in place of a govendor.toml manifest.
pkgs.buildGoApplication {
  inherit go;

  pname = "lucky-dip";
  version = "0.1.0";
  src = ./.;
}
```

The `vendor/` directory is generated with `go mod vendor` and committed alongside the source. When no `modules` parameter is provided, `buildGoApplication` detects it automatically.
