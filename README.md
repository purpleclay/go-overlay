# go-overlay

[![Auto-Update](https://github.com/purpleclay/go-overlay/actions/workflows/auto-update.yml/badge.svg)](https://github.com/purpleclay/go-overlay/actions/workflows/auto-update.yml)
![Nix](https://img.shields.io/badge/Nix-5277C3?logo=nixos&logoColor=white)
![Go](https://img.shields.io/badge/Go-00ADD8?logo=go&logoColor=white)

A Nix overlay for Go development. Pure[^1], reproducible[^2], and auto-updated[^3].

[^1]: No side effects—builds depend only on declared inputs, not system state.

[^2]: Given the same inputs, builds produce byte-for-byte identical outputs. Pin a Go version today and get the exact same binary in 5 years.

[^3]: GitHub Actions monitors [go.dev](https://go.dev/dl/) every 4 hours. When a new release is detected, a manifest is generated and committed automatically—no manual intervention required.

[^4]: gomod2nix's builder depends on its CLI tool, which in turn depends on the builder—complicating [NUR integration and bootstrapping](https://github.com/nix-community/gomod2nix/issues/196). go-overlay avoids this by having `govendor` and `buildGoApplication` communicate only via the manifest file.

[^5]: Go workspaces (`go.work`) allow multiple modules to be developed together in a monorepo. Neither [buildGoModule](https://github.com/NixOS/nixpkgs/issues/203039) nor [gomod2nix](https://discourse.nixos.org/t/gomod2nix-with-go-workspaces/43134) support workspaces because `-mod=vendor` conflicts with workspace mode. go-overlay's `buildGoWorkspace` works around this limitation.

- [Why it exists?](#why-it-exists)
- [Quick Start](#quick-start)
- [Installation](#installation)
- [Library Functions](#library-functions)
- [Builder Functions](#builder-functions)
  - [buildGoApplication](#buildgoapplication)
    - [In-tree Vendor Mode](#in-tree-vendor-mode)
    - [Manifest Mode](#manifest-mode)
    - [Local Replace Directives](#local-replace-directives)
    - [Proxy Configuration](#proxy-configuration)
    - [Cross-Compilation](#cross-compilation)
  - [buildGoWorkspace](#buildgoworkspace)
    - [In-tree Vendor Mode](#in-tree-vendor-mode-1)
    - [Manifest Mode](#manifest-mode-1)
    - [Workspace Structure](#workspace-structure)
    - [Generating the Manifest](#generating-the-manifest)
    - [Building Multiple Binaries](#building-multiple-binaries)
    - [Proxy Configuration](#proxy-configuration-1)
    - [Cross-Compilation](#cross-compilation-1)
  - [mkVendorEnv](#mkvendorenv)
- [Building a Go Application](#building-a-go-application)
- [Detecting Drift with Git Hooks](#detecting-drift-with-git-hooks)
- [Private Modules](#private-modules)
- [Using with buildGoModule](#using-with-buildgomodule)
- [Migration Guides](#migration-guides)
  - [From gomod2nix](#from-gomod2nix)
  - [From buildGoModule](#from-buildgomodule)
- [Used by](#used-by)

## Why it exists?

| Feature                  | go-overlay           | gomod2nix     | nixpkgs (buildGoModule) |
| :----------------------- | :------------------- | :------------ | :---------------------- |
| Go versions available    | 100+ (1.17 – latest) | nixpkgs only  | nixpkgs only            |
| New release availability | Up to 4 hours        | Days to weeks | Days to weeks           |
| Release candidates       | Yes                  | No            | No                      |
| vendorHash required      | No                   | No            | Yes                     |
| Unpatched Go binary      | Yes                  | No            | No                      |
| Go workspaces[^5]        | Yes                  | No            | No                      |
| Private modules          | Standard Go auth     | Complex setup | Complex setup           |
| Drift detection          | Yes (`--check`)      | No            | N/A                     |
| Circular dependency[^4]  | No                   | Yes           | N/A                     |

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

## Builder Functions

go-overlay provides builder functions for Go applications using vendored dependencies. Unlike nixpkgs' `buildGoModule`, these work with unpatched Go binaries from [go.dev](https://go.dev/dl/) and don't require computing a `vendorHash`.

### `buildGoApplication`

Build a Go application using vendored dependencies. Supports two modes:

1. **In-tree vendor**: Use an existing `vendor/` directory from your source
2. **Manifest mode**: Generate vendor from a `govendor.toml` manifest

**Use when:** You want reproducible Go builds without the `vendorHash` dance.

#### In-tree Vendor Mode

If your project already has a committed `vendor/` directory, simply omit the `modules` parameter:

```nix
buildGoApplication {
  pname = "my-app";
  version = "1.0.0";
  src = ./.;
  go = pkgs.go-bin.latest;
  subPackages = [ "cmd/my-app" ];
  # No modules parameter - uses vendor/ from src
}
```

#### Manifest Mode

Use a `govendor.toml` manifest for dependency management:

```nix
buildGoApplication {
  pname = "my-app";
  version = "1.0.0";
  src = ./.;
  go = pkgs.go-bin.latest;
  modules = ./govendor.toml;
  subPackages = [ "cmd/my-app" ];
}
```

| Option        | Default             | Description                                                |
| :------------ | :------------------ | :--------------------------------------------------------- |
| `pname`       | required            | Package name                                               |
| `version`     | required            | Package version                                            |
| `src`         | required            | Source directory                                           |
| `go`          | required            | Go derivation from go-overlay                              |
| `modules`     | `null`              | Path to govendor.toml manifest (null = use in-tree vendor) |
| `subPackages` | `["."]`             | Packages to build (relative to src)                        |
| `ldflags`     | `[]`                | Linker flags                                               |
| `tags`        | `[]`                | Build tags                                                 |
| `CGO_ENABLED` | inherited from `go` | Enable CGO                                                 |
| `GOOS`        | inherited from `go` | Target operating system                                    |
| `GOARCH`      | inherited from `go` | Target architecture                                        |
| `GOPROXY`     | `"off"`             | Go module proxy URL                                        |
| `GOPRIVATE`   | `""`                | Glob patterns for private modules                          |
| `GOSUMDB`     | `"off"`             | Checksum database URL                                      |
| `GONOSUMDB`   | `""`                | Glob patterns to skip checksum verification                |

#### Local Replace Directives

go-overlay supports local replace directives in `go.mod`:

```
replace example.com/mylib => ./libs/mylib
```

When `govendor` detects a local replacement, it records the path in `govendor.toml`:

```toml
[mod."example.com/mylib"]
  version = "v1.0.0"
  hash = "sha256-..."
  replaced = "example.com/mylib"
  local = "./libs/mylib"
```

During the build, `buildGoApplication` copies the local module from your source tree into the vendor directory. This works automatically—no additional configuration required.

#### Proxy Configuration

By default, `buildGoApplication` sets `GOPROXY=off` and `GOSUMDB=off` since dependencies are vendored. However, you can override these for corporate proxies or private module servers:

```nix
buildGoApplication {
  pname = "myapp";
  version = "1.0.0";
  src = ./.;
  go = pkgs.go-bin.latest;
  modules = ./govendor.toml;

  # Corporate proxy with fallback
  GOPROXY = "https://proxy.corp.example.com,https://proxy.golang.org,direct";
  GOPRIVATE = "github.com/myorg/*";
  GOSUMDB = "sum.golang.org";
}
```

#### Cross-Compilation

Build binaries for different platforms by overriding `GOOS` and `GOARCH`:

```nix
buildGoApplication {
  pname = "myapp";
  version = "1.0.0";
  src = ./.;
  go = pkgs.go-bin.latest;
  modules = ./govendor.toml;

  # Build for Windows on Linux/macOS
  GOOS = "windows";
  GOARCH = "amd64";
  CGO_ENABLED = 0;
}
```

By default, govendor resolves dependencies for these platforms:

- `linux/amd64`, `linux/arm64`
- `darwin/amd64`, `darwin/arm64`
- `windows/amd64`, `windows/arm64`

To build for platforms outside the defaults (e.g., FreeBSD), use `--include-platform` when generating the manifest:

```bash
govendor --include-platform=freebsd/amd64
```

This ensures dependencies with platform-specific build constraints are included. The additional platforms are persisted in `govendor.toml` and automatically used on subsequent runs.

For multi-platform releases:

```nix
let
  platforms = [
    { goos = "linux"; goarch = "amd64"; }
    { goos = "darwin"; goarch = "amd64"; }
    { goos = "freebsd"; goarch = "amd64"; }
  ];
in
builtins.listToAttrs (map (p: {
  name = "myapp-${p.goos}-${p.goarch}";
  value = pkgs.buildGoApplication {
    pname = "myapp";
    version = "1.0.0";
    src = ./.;
    go = pkgs.go-bin.latest;
    modules = ./govendor.toml;
    GOOS = p.goos;
    GOARCH = p.goarch;
    CGO_ENABLED = 0;
  };
}) platforms)
```

### `buildGoWorkspace`

Build applications from a [Go workspace](https://go.dev/doc/tutorial/workspaces) (`go.work` file). Use this when your project is a monorepo with multiple Go modules that share dependencies. Supports two modes:

1. **In-tree vendor**: Use an existing `vendor/` directory from `go work vendor`
2. **Manifest mode**: Generate vendor from a `govendor.toml` manifest

**Use when:** You have a `go.work` file coordinating multiple modules in a single repository.

#### In-tree Vendor Mode

If your workspace already has a committed `vendor/` directory (from `go work vendor`), simply omit the `modules` parameter:

```nix
buildGoWorkspace {
  pname = "api";
  version = "1.0.0";
  src = ./.;
  go = pkgs.go-bin.latest;
  subPackages = [ "api" ];
  # No modules parameter - uses vendor/ from src
}
```

#### Manifest Mode

Use a `govendor.toml` manifest for dependency management:

```nix
buildGoWorkspace {
  pname = "api";
  version = "1.0.0";
  src = ./.;
  go = pkgs.go-bin.latest;
  modules = ./govendor.toml;
  subPackages = [ "api" ];
}
```

#### Workspace Structure

A typical workspace might look like:

```
my-monorepo/
├── go.work
├── govendor.toml
├── api/
│   ├── go.mod
│   └── main.go
├── worker/
│   ├── go.mod
│   └── main.go
└── shared/
    ├── go.mod
    └── lib.go
```

Where `go.work` contains:

```
go 1.22

use (
    ./api
    ./worker
    ./shared
)
```

#### Generating the Manifest

Run `govendor` in the workspace root (where `go.work` lives):

```bash
govendor
```

This generates a `govendor.toml` that includes:
- External dependencies with NAR hashes
- Workspace modules that are dependencies of other modules

#### Building Multiple Binaries

Build each application separately using the same manifest:

```nix
# default.nix
{ buildGoWorkspace, go }:
{
  api = buildGoWorkspace {
    pname = "api";
    version = "1.0.0";
    src = ./.;
    modules = ./govendor.toml;
    subPackages = [ "api" ];
    inherit go;
  };

  worker = buildGoWorkspace {
    pname = "worker";
    version = "1.0.0";
    src = ./.;
    modules = ./govendor.toml;
    subPackages = [ "worker" ];
    inherit go;
  };
}
```

| Option        | Default             | Description                                                |
| :------------ | :------------------ | :--------------------------------------------------------- |
| `pname`       | required            | Package name                                               |
| `version`     | required            | Package version                                            |
| `src`         | required            | Source directory (workspace root)                          |
| `go`          | required            | Go derivation from go-overlay                              |
| `modules`     | `null`              | Path to govendor.toml manifest (null = use in-tree vendor) |
| `subPackages` | `["."]`             | Packages to build (relative to workspace root)             |
| `ldflags`     | `[]`                | Linker flags                                               |
| `tags`        | `[]`                | Build tags                                                 |
| `CGO_ENABLED` | inherited from `go` | Enable CGO                                                 |
| `GOOS`        | inherited from `go` | Target operating system                                    |
| `GOARCH`      | inherited from `go` | Target architecture                                        |
| `GOPROXY`     | `"off"`             | Go module proxy URL                                        |
| `GOPRIVATE`   | `""`                | Glob patterns for private modules                          |
| `GOSUMDB`     | `"off"`             | Checksum database URL                                      |
| `GONOSUMDB`   | `""`                | Glob patterns to skip checksum verification                |

#### Proxy Configuration

By default, `buildGoWorkspace` sets `GOPROXY=off` and `GOSUMDB=off` since dependencies are vendored. However, you can override these for corporate proxies or private module servers:

```nix
buildGoWorkspace {
  pname = "api";
  version = "1.0.0";
  src = ./.;
  go = pkgs.go-bin.latest;
  modules = ./govendor.toml;
  subPackages = [ "api" ];

  # Corporate proxy with fallback
  GOPROXY = "https://proxy.corp.example.com,https://proxy.golang.org,direct";
  GOPRIVATE = "github.com/myorg/*";
  GOSUMDB = "sum.golang.org";
}
```

#### Cross-Compilation

Build binaries for different platforms by overriding `GOOS` and `GOARCH`:

```nix
buildGoWorkspace {
  pname = "api";
  version = "1.0.0";
  src = ./.;
  go = pkgs.go-bin.latest;
  modules = ./govendor.toml;
  subPackages = [ "api" ];

  # Build for Windows on Linux/macOS
  GOOS = "windows";
  GOARCH = "amd64";
  CGO_ENABLED = 0;
}
```

By default, govendor resolves dependencies for these platforms:

- `linux/amd64`, `linux/arm64`
- `darwin/amd64`, `darwin/arm64`
- `windows/amd64`, `windows/arm64`

To build for platforms outside the defaults (e.g., FreeBSD), use `--include-platform` when generating the manifest:

```bash
govendor --include-platform=freebsd/amd64
```

### `mkVendorEnv`

Create a vendor directory with `modules.txt` from a parsed `govendor.toml` manifest. This is a lower-level function used internally by `buildGoApplication`.

**Use when:** You need custom control over the vendor directory or build process—for example, when integrating with code generation, custom build steps, or existing `stdenv.mkDerivation` workflows.

```nix
mkVendorEnv {
  go = pkgs.go-bin.latest;
  manifest = builtins.fromTOML (builtins.readFile ./govendor.toml);
}
```

| Option     | Default  | Description                                          |
| :--------- | :------- | :--------------------------------------------------- |
| `go`       | required | Go derivation from go-overlay                        |
| `manifest` | required | Parsed govendor.toml (via fromTOML)                  |
| `src`      | `null`   | Source tree (required if manifest has local modules) |

The resulting derivation contains each module at its import path and a `modules.txt` with package listings.

#### Custom Build Example

```nix
{ pkgs }:

let
  go = pkgs.go-bin.latest;
  vendorEnv = pkgs.mkVendorEnv {
    inherit go;
    manifest = builtins.fromTOML (builtins.readFile ./govendor.toml);
  };
in
pkgs.stdenv.mkDerivation {
  pname = "myapp";
  version = "1.0.0";
  src = ./.;

  nativeBuildInputs = [ go ];

  configurePhase = ''
    export GOCACHE=$TMPDIR/go-cache
    export GOPATH=$TMPDIR/go
    cp -r ${vendorEnv} vendor
    chmod -R u+w vendor
  '';

  buildPhase = ''
    go build -mod=vendor -o myapp ./cmd/myapp
  '';

  installPhase = ''
    mkdir -p $out/bin
    cp myapp $out/bin/
  '';
}
```

#### With Code Generation

For projects requiring code generation before building:

```nix
{ pkgs }:

let
  go = pkgs.go-bin.latest;
  vendorEnv = pkgs.mkVendorEnv {
    inherit go;
    manifest = builtins.fromTOML (builtins.readFile ./govendor.toml);
  };
in
pkgs.stdenv.mkDerivation {
  pname = "myapp";
  version = "1.0.0";
  src = ./.;

  nativeBuildInputs = [ go pkgs.protobuf pkgs.protoc-gen-go ];

  configurePhase = ''
    export GOCACHE=$TMPDIR/go-cache
    export GOPATH=$TMPDIR/go
    cp -r ${vendorEnv} vendor
    chmod -R u+w vendor
  '';

  buildPhase = ''
    # Generate code first
    protoc --go_out=. proto/*.proto

    # Then build
    go build -mod=vendor -o myapp ./cmd/myapp
  '';

  installPhase = ''
    mkdir -p $out/bin
    cp myapp $out/bin/
  '';
}
```

## Building a Go Application

### Step 1: Add govendor to Your Dev Shell

Add `govendor` to your development shell to generate vendor manifests:

```nix
# flake.nix
{
  inputs.go-overlay.url = "github:purpleclay/go-overlay";

  outputs = { self, nixpkgs, go-overlay, ... }:
    let
      pkgs = import nixpkgs {
        system = "x86_64-linux";
        overlays = [ go-overlay.overlays.default ];
      };
    in {
      devShells.default = pkgs.mkShell {
        buildInputs = [
          pkgs.go-bin.fromGoMod ./go.mod
          go-overlay.packages.${pkgs.system}.govendor
        ];
      };
    };
}
```

### Step 2: Generate a Vendor Manifest

Run `govendor` to generate a `govendor.toml` manifest:

```bash
govendor
```

This creates a `govendor.toml` file with NAR hashes for all dependencies. Commit this file to your repository.

> [!TIP]
> Re-run `govendor` whenever your dependencies change. Use `govendor --check` in CI to detect manifest drift, or set up a [git hook](#detecting-drift-with-git-hooks) to catch drift before committing.

### Step 3: Create a Package Definition

Create a `default.nix` file to build your application:

```nix
# default.nix
{
  buildGoApplication,
  go,
}:
buildGoApplication {
  pname = "my-app";
  version = "1.0.0";
  src = ./.;
  inherit go;
  subPackages = [ "cmd/my-app" ];
  ldflags = [ "-s" "-w" ];
}
```

### Step 4: Build Your Application

Add the package to your flake outputs:

```nix
# flake.nix
{
  packages.default = pkgs.callPackage ./default.nix {
    inherit (pkgs) buildGoApplication;
    go = pkgs.go-bin.fromGoMod ./go.mod;
  };
}
```

Build with:

```bash
nix build
```

## Detecting Drift with Git Hooks

Use [cachix/git-hooks.nix](https://github.com/cachix/git-hooks.nix) to automatically check for manifest drift when `go.mod` or `go.work` changes:

```nix
# flake.nix
{
  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
    go-overlay.url = "github:purpleclay/go-overlay";
    git-hooks.url = "github:cachix/git-hooks.nix";
  };

  outputs = { self, nixpkgs, go-overlay, git-hooks, ... }:
    let
      system = "x86_64-linux";
      pkgs = import nixpkgs {
        inherit system;
        overlays = [ go-overlay.overlays.default ];
      };

      pre-commit-check = git-hooks.lib.${system}.run {
        src = ./.;
        hooks = {
          govendor = {
            enable = true;
            name = "govendor";
            description = "Check if govendor.toml has drifted from go.mod or go.work";
            entry = "${go-overlay.packages.${system}.govendor}/bin/govendor --check";
            files = "(^|/)go\\.(mod|work)$";
            pass_filenames = true;
          };
        };
      };
    in {
      devShells.default = pkgs.mkShell {
        inherit (pre-commit-check) shellHook;
        buildInputs = pre-commit-check.enabledPackages;
      };
    };
}
```

When you modify `go.mod` or `go.work` and attempt to commit, the hook will fail if `govendor.toml` is out of sync:

```
govendor.................................................................Failed
- hook id: govendor
- exit code: 1

╭────────┬─────────┬──────────────────────────────────────────────╮
│ File   │ Status  │ Message                                      │
├────────┼─────────┼──────────────────────────────────────────────┤
│ go.mod │ ✗ drift │ go.mod has changed, regenerate govendor.toml │
╰────────┴─────────┴──────────────────────────────────────────────╯
```

Run `govendor` to regenerate the manifest, then commit both files together.

## Private Modules

go-overlay supports private Go modules through standard Go authentication mechanisms.

### Generating Manifests

When running `govendor`, configure authentication via environment variables or `.netrc`:

```bash
# Set GOPRIVATE to bypass the checksum database
export GOPRIVATE="github.com/myorg/*,gitlab.mycompany.com/*"

# Configure git to use token authentication
git config --global url."https://${GITHUB_TOKEN}@github.com/".insteadOf "https://github.com/"

# Generate manifest
govendor
```

Alternatively, use `~/.netrc`:

```
machine github.com
  login oauth2
  password ghp_xxxxxxxxxxxx
```

### Building with Private Modules

Configure `buildGoApplication` with the appropriate environment variables:

```nix
buildGoApplication {
  pname = "myapp";
  version = "1.0.0";
  src = ./.;
  go = pkgs.go-bin.latest;

  GOPRIVATE = "github.com/myorg/*";
  GOPROXY = "https://proxy.golang.org,direct";
}
```

### Using a Private Proxy

For organizations running Athens, Artifactory, or similar:

```nix
buildGoApplication {
  pname = "myapp";
  version = "1.0.0";
  src = ./.;
  go = pkgs.go-bin.latest;

  GOPROXY = "https://athens.mycompany.com";
  GOSUMDB = "off";
}
```

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

## Migration Guides

### From gomod2nix

#### Before (gomod2nix)

```nix
# flake.nix
{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    gomod2nix.url = "github:nix-community/gomod2nix";
  };

  outputs = { nixpkgs, gomod2nix, ... }:
    let
      pkgs = import nixpkgs {
        system = "x86_64-linux";
        overlays = [ gomod2nix.overlays.default ];
      };
    in {
      packages.default = pkgs.buildGoApplication {
        pname = "myapp";
        version = "1.0.0";
        src = ./.;
        modules = ./gomod2nix.toml;
      };
    };
}
```

```bash
gomod2nix generate
```

#### After (go-overlay)

```nix
# flake.nix
{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    go-overlay.url = "github:purpleclay/go-overlay";
  };

  outputs = { nixpkgs, go-overlay, ... }:
    let
      system = "x86_64-linux";
      pkgs = import nixpkgs {
        inherit system;
        overlays = [ go-overlay.overlays.default ];
      };
    in {
      packages.default = pkgs.buildGoApplication {
        pname = "myapp";
        version = "1.0.0";
        src = ./.;
        go = pkgs.go-bin.fromGoMod ./go.mod;
        modules = ./govendor.toml;
      };
    };
}
```

```bash
govendor
```

#### Migration Steps

1. Replace gomod2nix with go-overlay in flake inputs
2. Update the overlay reference
3. Add `go` parameter to `buildGoApplication`
4. Run `govendor` to generate the new manifest
5. Delete `gomod2nix.toml` and commit `govendor.toml`

### From buildGoModule

#### Before (buildGoModule)

```nix
# flake.nix
{
  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { nixpkgs, ... }:
    let
      pkgs = nixpkgs.legacyPackages.x86_64-linux;
    in {
      packages.default = pkgs.buildGoModule {
        pname = "myapp";
        version = "1.0.0";
        src = ./.;
        vendorHash = "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=";
      };
    };
}
```

#### After (go-overlay)

```nix
# flake.nix
{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    go-overlay.url = "github:purpleclay/go-overlay";
  };

  outputs = { nixpkgs, go-overlay, ... }:
    let
      system = "x86_64-linux";
      pkgs = import nixpkgs {
        inherit system;
        overlays = [ go-overlay.overlays.default ];
      };
    in {
      packages.default = pkgs.buildGoApplication {
        pname = "myapp";
        version = "1.0.0";
        src = ./.;
        go = pkgs.go-bin.fromGoMod ./go.mod;
        modules = ./govendor.toml;
      };
    };
}
```

```bash
govendor
```

#### Migration Steps

1. Add go-overlay to flake inputs
2. Add overlay to pkgs
3. Replace `buildGoModule` with `buildGoApplication`
4. Remove `vendorHash` parameter
5. Add `go` and `modules` parameters
6. Run `govendor` to generate the manifest
7. Commit `govendor.toml`

## Used By

- [devenv](https://github.com/cachix/devenv) - Fast, declarative, reproducible developer environments
