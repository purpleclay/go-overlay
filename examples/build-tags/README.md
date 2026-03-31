# build-tags

A quote-of-the-day CLI that changes its personality and colour based on Go build tags. The same source, compiled three different ways.

## Getting started

```shell
# Default — yellow, no strong opinions
nix run .#example-build-tags

# Meaningful — green
nix run .#example-build-tags-meaningful

# Procrastination — blue
nix run .#example-build-tags-procrastination
```

## The Nix bit

```nix
{
  pkgs,
  go,
  tags ? [],
}:
pkgs.buildGoApplication {
  inherit go tags;

  pname = "build-tags";
  version = "0.1.0";
  src = ./.;
  modules = ./govendor.toml;
}
```

The `default.nix` is tag-agnostic — the variation is driven entirely from the caller:

```nix
example-build-tags             = import ./build-tags {inherit pkgs go;};
example-build-tags-meaningful  = import ./build-tags {inherit pkgs go; tags = ["meaningful"];};
example-build-tags-procrastination = import ./build-tags {inherit pkgs go; tags = ["procrastination"];};
```

Each `quotes_*.go` file carries a `//go:build` constraint alongside its own colour — only one file is compiled into the binary, so the tag controls both the quotes and the output colour.
