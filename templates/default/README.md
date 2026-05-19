# Go Application

A basic Go application bootstrapped with [go-overlay](https://github.com/purpleclay/go-overlay).

## Getting Started

Build the application:

```shell
nix build
./result/bin/example
```

> [!TIP]
> Run `nix build -L` to print full build logs and see each phase as it happens.

Or run the application directly:

```shell
nix run
```

## Developer Shell

Enter the development shell with the Go toolchain and `govendor` pre-installed:

```shell
nix develop
```
