# go-overlay

Pure and reproducible nix overlay of binary distributed golang toolchains. Current oldest supported toolchain is 1.17, the latest version is always auto-updated through GitHub Actions.

## Quick Start

Try Go without installing anything permanently:

- Run the latest Go toolchain directly:
  ```sh
  $ nix run github:purpleclay/go-overlay -- version
  go version go1.25.5 linux/amd64
  ```
- Enter a shell with the latest Go toolchain available:
  ```sh
  $ nix shell github:purpleclay/go-overlay
  $ go version
  go version go1.25.5 linux/amd64
  ```
- Build the latest Go toolchain:
  ```sh
  $ nix build github:purpleclay/go-overlay
  ./result/bin/go version
  go version go1.25.5 linux/amd64
  ```
- Select a specific version of Go through the exposed packages. A package takes on the format `go_<major>_<minor>_<patch>`:
  ```sh
  $ nix shell github:purpleclay/go-overlay#go_1_23_2 -- version
  go version go1.23.2 linux/amd64
  ```

## Installation

### Nix Flakes

Running `nix develop` will enter a shell with the latest version of Go installed.

```nix
{
  description = "A Go-Overlay DevShell Example";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
    go-overlay.url = "github:purpleclay/go-overlay";
  };

  outputs = { nixpkgs, go-overlay, flake-utils, ... }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs {
          inherit system;
          overlays = [ go-overlay.overlays.default ];
        };
      in
      {
        devShells.default = with pkgs; mkShell {
          buildInputs = [ go-bin.latest ];
        };
      }
    );
}
```

```sh
$ nix develop
$ go version

# newest version at time of writing
go version go1.25.5 linux/amd64
```

## Cheat Sheet

Discover the common usage patterns for `go-bin`:

- Always select the latest version of Go (_includes release candidates in selection_):

```nix
go-bin.latest
```

- Lock to a specific version of Go for pure reproducibility:

```nix
go-bin.versions."1.17.2"
go-bin.versions."1.21.4"
go-bin.versions."1.25.4"
```

- Select Go version based on `go.mod` (uses `toolchain` directive if present, otherwise latest patch of `go` directive):

```nix
go-bin.fromGoMod ./go.mod
```

- Select exact Go version from `go.mod` (no latest patch fallback, fails if version unavailable):

```nix
go-bin.fromGoModStrict ./go.mod
```

| go.mod                           | `fromGoMod`   | `fromGoModStrict` |
| -------------------------------- | ------------- | ----------------- |
| `go 1.21`                        | Latest 1.21.x | Error             |
| `go 1.21.6`                      | 1.21.6        | 1.21.6            |
| `go 1.21` + `toolchain go1.21.6` | 1.21.6        | 1.21.6            |

## Using with buildGoModule

To use go-overlay with nixpkgs' `buildGoModule`, you must use `.override` to replace the Go toolchain. Passing `go` as a build argument will **not work**, as it will default to nixpkgs' `go` package.

```nix
# default.nix
{
  pkgs,
  go,
}:
(pkgs.buildGoModule.override {inherit go;}) {
  pname = "my-app";
  version = "1.0.0";
  src = ./.;
  vendorHash = "sha256-...";
}
```

```nix
# flake.nix
packages.my-app = import ./. {
  inherit pkgs;
  go = pkgs.go-bin.versions."1.22.3";
};
```
