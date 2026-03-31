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
  modules = ./govendor.toml;
  
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

`fatih/color`, which pulls in `go-isatty` and `go-colorable` — both have platform-specific implementations for Windows. Running `govendor` with `--include-platform` extends dependency resolution beyond the host:

```shell
govendor --include-platform freebsd/amd64 --include-platform windows/amd64
```

The platforms are recorded in `govendor.toml` so the resolution is reproducible without re-running the flags:

```toml
include_platforms = ["freebsd/amd64", "windows/amd64"]
```
