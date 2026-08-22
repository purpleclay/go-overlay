# cross-compile

A small CLI that tells you about your OS and passes judgement on your choices. The same source, compiled for Linux, FreeBSD, and Windows from a single macOS build.

## Getting started

```shell
# Run on the current machine
nix run .#example-cross-compile

# Build for Linux (ELF binary)
nix build .#example-cross-compile-linux

# Build for FreeBSD (FreeBSD ELF binary)
nix build .#example-cross-compile-freebsd

# Build for Windows (PE32+ binary)
nix build .#example-cross-compile-windows
```

Cross-compiled outputs land in a platform-prefixed subdirectory (`bin/$GOOS_$GOARCH/`). Confirm the binary format with `file`:

```shell
file result/bin/freebsd_amd64/cross-compile
# result/bin/freebsd_amd64/cross-compile: ELF 64-bit LSB executable, x86-64, version 1 (FreeBSD), statically linked

file result/bin/windows_amd64/cross-compile.exe
# result/bin/windows_amd64/cross-compile.exe: PE32+ executable (console) x86-64, for MS Windows
```

## The Nix bit

```nix
{
  pkgs,
  go,
  GOOS ? go.GOOS,
  GOARCH ? go.GOARCH,
}:
pkgs.buildGoApplication {
  inherit go GOOS GOARCH;

  pname = "cross-compile";
  version = "0.1.0";
  src = ./.;

  # CGO_ENABLED = "0" is required for cross-compilation. Pure-Go binaries need
  # no C cross-compiler, no sysroot — just the Go toolchain.
  CGO_ENABLED = "0";
}
```

The targets are driven entirely from `examples/default.nix`:

```nix
example-cross-compile  = import ./cross-compile {inherit pkgs go;};
example-cross-compile-linux   = import ./cross-compile {inherit pkgs go; GOOS = "linux";   GOARCH = "amd64";};
example-cross-compile-freebsd = import ./cross-compile {inherit pkgs go; GOOS = "freebsd"; GOARCH = "amd64";};
example-cross-compile-windows = import ./cross-compile {inherit pkgs go; GOOS = "windows"; GOARCH = "amd64";};
```

## The govendor bit

This example depends on `fatih/color`, which pulls in `go-isatty` and `go-colorable` — both with platform-specific implementations for Windows. There is nothing to configure for this: dependency resolution is platform-independent, so a plain run covers every target this example builds for (and every one it doesn't):

```shell
govendor
```

The Windows-only packages appear in `govendor.toml` alongside everything else — no flags, nothing extra persisted:

```toml
[mod."github.com/mattn/go-colorable"]
  version = "v0.1.13"
  hash = "sha256-..."
  go = "1.15"
  packages = ["github.com/mattn/go-colorable"]
```

> [!NOTE]
> Before schema v4, targets beyond the defaults required `govendor --include-platform freebsd/amd64 --include-platform windows/amd64`. That flag no longer exists — see the [migration guide](../../docs/migrating.md#from-schema-v3-to-v4).
