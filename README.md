# go-overlay

[![Auto-Update](https://github.com/purpleclay/go-overlay/actions/workflows/auto-update.yml/badge.svg)](https://github.com/purpleclay/go-overlay/actions/workflows/auto-update.yml)
![Nix](https://img.shields.io/badge/Nix-5277C3?logo=nixos&logoColor=white)
![Go](https://img.shields.io/badge/Go-00ADD8?logo=go&logoColor=white)

Nix overlay for Go toolchains. Pure[^1], reproducible[^2], auto-updated[^3] binaries for Go 1.17+. No installation needed.

[^1]: No side effects—builds depend only on declared inputs, not system state.

[^2]: Given the same inputs, builds produce byte-for-byte identical outputs. Pin a Go version today and get the exact same binary in 5 years.

[^3]: GitHub Actions monitors [go.dev](https://go.dev/dl/) every 4 hours. When a new release is detected, a manifest is generated and committed automatically—no manual intervention required.

- [Why it exists?](#why-it-exists)
- [Quick Start](#quick-start)
- [Installation](#installation)
- [Library Functions](#library-functions)
- [Using with buildGoModule](#using-with-buildgomodule)
- [Migrating from nixpkgs](#migrating-from-nixpkgs)
- [Limitations](#limitations)
- [Used by](#used-by)

## Why it exists?

| Feature                  | go-overlay                   | nixpkgs                        |
| ------------------------ | ---------------------------- | ------------------------------ |
| Versions available       | 100+ (1.17 – latest)         | 2 per nixpkgs commit           |
| New release availability | Up to 4 hours after upstream | Days to weeks                  |
| Multiple versions        | Single flake input           | Multiple nixpkgs pins required |
| Release candidates       | Available                    | Not available                  |

> [!NOTE]
> Older Go versions _are_ accessible in nixpkgs by pinning historical commits, but this requires managing multiple nixpkgs inputs and finding the correct commit for each version.

## Quick Start

Try Go without installing anything permanently.

### Direct Execution

Run Go without any setup:

```bash
nix run github:purpleclay/go-overlay -- version
# go version go1.25.5 linux/amd64
```

### Shell Environment

Interactive development:

```bash
nix shell github:purpleclay/go-overlay
go version
# go version go1.25.5 linux/amd64
```

### Build Output

Create a derivation:

```bash
nix build github:purpleclay/go-overlay
./result/bin/go version
# go version go1.25.5 linux/amd64
```

### Specific Version

Pin to a known version using the format `go_<major>_<minor>_<patch>`:

```bash
nix shell github:purpleclay/go-overlay#go_1_23_2
go version
# go version go1.23.2 linux/amd64
```

## Installation

### Nix Flakes

Add go-overlay to your flake inputs and apply the overlay:

```nix
{
  description = "My Go Project";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
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
        devShells.default = pkgs.mkShell {
          buildInputs = [ pkgs.go-bin.latest ];
        };
      }
    );
}
```

```bash
nix develop
go version
# go version go1.25.5 linux/amd64
```

### Traditional Nix (non-flake)

For users not using flakes, go-overlay can be imported directly as an overlay.

> [!TIP]
> For reproducible builds, pin to a specific commit instead of `main`:
>
> ```nix
> builtins.fetchTarball "https://github.com/purpleclay/go-overlay/archive/<commit-sha>.tar.gz"
> ```
>
> Find commit SHAs [here](https://github.com/purpleclay/go-overlay/commits/main).

#### Option 1: Using fetchTarball

Direct import in your expression:

```nix
let
  go-overlay = import (builtins.fetchTarball
    "https://github.com/purpleclay/go-overlay/archive/main.tar.gz");

  pkgs = import <nixpkgs> {
    overlays = [ go-overlay ];
  };
in
pkgs.mkShell {
  buildInputs = [ pkgs.go-bin.latest ];
}
```

#### Option 2: User Overlays

Add to `~/.config/nixpkgs/overlays.nix`:

```nix
[
  (import (builtins.fetchTarball
    "https://github.com/purpleclay/go-overlay/archive/main.tar.gz"))
]
```

Then use in any expression:

```nix
let
  pkgs = import <nixpkgs> {};
in
pkgs.go-bin.latest
```

#### Option 3: Nix Channels

```bash
nix-channel --add https://github.com/purpleclay/go-overlay/archive/main.tar.gz go-overlay
nix-channel --update
```

Then import the channel:

```nix
let
  go-overlay = import <go-overlay>;

  pkgs = import <nixpkgs> {
    overlays = [ go-overlay ];
  };
in
pkgs.go-bin.latest
```

## Library Functions

### `go-bin.latest`

Get the absolute latest version, including release candidates.

**Use when:** You want cutting-edge features and don't mind pre-release software.

```nix
go-bin.latest
```

### `go-bin.latestStable`

Get the latest stable version, excluding release candidates. Recommended for production.

**Use when:** You need stability and don't require the latest features.

```nix
go-bin.latestStable
```

### `go-bin.versions.<version>`

Pin to an exact version for complete reproducibility.

**Use when:** You need deterministic builds and exact version control.

```nix
go-bin.versions."1.21.4"
go-bin.versions."1.25.4"
```

### `go-bin.hasVersion <version>`

Check if a specific version is available before using it.

**Use when:** You want to handle missing versions gracefully.

```nix
if go-bin.hasVersion "1.22.0"
then go-bin.versions."1.22.0"
else go-bin.latestStable
```

### `go-bin.isDeprecated <version>`

Check if a version is deprecated (EOL) according to [Go's release policy](https://go.dev/doc/devel/release#policy).

Go supports the current and previous minor versions. If the latest stable is 1.25.x:

```nix
go-bin.isDeprecated "1.23.4"  # true (two versions behind)
go-bin.isDeprecated "1.24.0"  # false (previous minor, supported)
go-bin.isDeprecated "1.25.0"  # false (current minor, supported)
```

### `go-bin.fromGoMod <path>`

Auto-select Go version from `go.mod`. Uses `toolchain` directive if present, otherwise the latest patch of the `go` directive.

**Use when:** You want automatic version selection based on your project.

```nix
go-bin.fromGoMod ./go.mod
```

### `go-bin.fromGoModStrict <path>`

Strict version matching from `go.mod`. No automatic patch version selection; fails if exact version is unavailable.

**Use when:** You need exact reproducibility and want early failure on version mismatch.

```nix
go-bin.fromGoModStrict ./go.mod
```

### Behavior Comparison

> [!NOTE]
> `fromGoMod` is flexible and forgiving—great for development. `fromGoModStrict` is strict and predictable—better for reproducible builds.

| go.mod Declaration               | `fromGoMod`   | `fromGoModStrict` |
| :------------------------------- | :------------ | :---------------- |
| `go 1.21`                        | Latest 1.21.x | Error             |
| `go 1.21.6`                      | 1.21.6        | 1.21.6            |
| `go 1.21` + `toolchain go1.21.6` | 1.21.6        | 1.21.6            |

## Using with buildGoModule

`buildGoModule` defaults to nixpkgs' Go toolchain. To use go-overlay, you must override it.

> [!WARNING]
> Simply passing `go` as an argument will **not work** because `buildGoModule` ignores build arguments for its Go dependency.

### Override in default.nix

```nix
# default.nix
{
  pkgs,
  go,
}:
(pkgs.buildGoModule.override { inherit go; }) {
  pname = "my-app";
  version = "1.0.0";
  src = ./.;
  vendorHash = "sha256-...";
}
```

### Override in flake.nix

```nix
# flake.nix
{
  packages.my-app = pkgs.callPackage ./default.nix {
    go = pkgs.go-bin.versions."1.22.3";
  };
}
```

## Migrating from nixpkgs

Migrating from nixpkgs to go-overlay involves changing how Go versions are specified in your Nix expressions.

### Before (nixpkgs)

```nix
# pin to minor version only
{
  buildInputs = [ pkgs.go_1_24 ];
}
```

### After (go-overlay)

```nix
# pin to exact version
{
  buildInputs = [ pkgs.go-bin.versions."1.24.5" ];
}
```

## Limitations

### Incompatible with buildGoApplication (gomod2nix)

go-overlay is **not compatible** with `buildGoApplication` from [gomod2nix](https://github.com/nix-community/gomod2nix).

#### Why?

Nixpkgs applies a [custom patch](https://github.com/NixOS/nixpkgs/blob/master/pkgs/development/compilers/go/go_no_vendor_checks-1.23.patch) to Go that adds support for the `GO_NO_VENDOR_CHECKS` environment variable. This patch allows `buildGoApplication` to skip vendor consistency checks when no `vendor/modules.txt` file is present. go-overlay uses pre-built binary distributions from [go.dev](https://go.dev/dl/), which do not include this patch. As a result, Go 1.14+ will fail with "inconsistent vendoring" errors when used with `buildGoApplication`.

#### Workaround:

Use `buildGoModule` instead of `buildGoApplication`. See [Using with buildGoModule](#using-with-buildgomodule) for details.

## Used By

- [devenv](https://github.com/cachix/devenv) - Fast, declarative, reproducible developer environments
