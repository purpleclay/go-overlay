# go-overlay

Pure and reproducible nix overlay of binary distributed golang toolchains. Current oldest supported toolchain is 1.17, the latest version is always auto-updated through GitHub Actions.

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

- Always select the latest version of Go:

```sh
go-bin.latest
```

- Lock to a specific version of Go for pure reproducibility:

```sh
go-bin.versions."1.17.2"
go-bin.versions."1.21.4"
go-bin.versions."1.25.4"
```
