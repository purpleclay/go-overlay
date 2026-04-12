<div align="center">
  <img src="https://github.com/purpleclay/go-overlay/raw/main/docs/static/go-overlay.png" width="240px" />
  <h1>go-overlay</h1>
  <p>A complete Go development environment for Nix.<br>Toolchains, tools, and builders — pure, reproducible, and auto-updated.</p>
  <img alt="Nix" src="https://img.shields.io/badge/Nix-5277C3?logo=nixos&logoColor=white" />
  <img alt="Go" src="https://img.shields.io/badge/Go-00ADD8?logo=go&logoColor=white" />
  <a href="LICENSE"><img alt="MIT" src="https://img.shields.io/badge/MIT-gray?logo=github&logoColor=white" /></a>
  <a href="https://github.com/purpleclay/go-overlay/actions/workflows/go-update.yml"><img alt="Go Update" src="https://github.com/purpleclay/go-overlay/actions/workflows/go-update.yml/badge.svg" /></a>
</div>
<br>
<br>

- **100+ Go versions** from 1.17 to latest, including release candidates.
- **New releases within 4 hours** of appearing on [go.dev](https://go.dev/dl/).
- **Go tools pinned to your toolchain** — govulncheck, gopls, golangci-lint, and more.
- **No `vendorHash`** — dependencies are tracked per-module with NAR hashes.
- **Workspace support** — build from `go.work` monorepos.
- **Private modules** — standard Go authentication via `.netrc`.
- **Unpatched Go binaries** — direct from [go.dev](https://go.dev/dl/), not rebuilt by Nix.

## Quick Start

### Try Go without installing anything

```bash
nix run github:purpleclay/go-overlay -- version
# go version go1.25.5 linux/amd64
```

### Start a new project

The template bootstraps a flake with a dev shell, builder, and vendored dependencies:

```bash
nix flake new -t github:purpleclay/go-overlay my-app
cd my-app
nix develop
```

### Add to an existing project

```bash
nix flake init -t github:purpleclay/go-overlay
```

## Installation

Add go-overlay to your flake inputs and apply the overlay:

```nix
{
  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
    go-overlay.url = "github:purpleclay/go-overlay";
  };

  outputs = { nixpkgs, go-overlay, ... }:
    let
      pkgs = import nixpkgs {
        system = "x86_64-linux";
        overlays = [ go-overlay.overlays.default ];
      };
    in {
      devShells.default = pkgs.mkShell {
        buildInputs = [ pkgs.go-bin.latest ];
      };
    };
}
```

> [!TIP]
> For traditional Nix (non-flake) installation options, see [Reference](docs/reference.md#traditional-nix).

## Choosing a Go Version

```nix
pkgs.go-bin.latest              # Absolute latest, including RCs
pkgs.go-bin.latestStable        # Latest stable release
pkgs.go-bin.versions."1.22.3"   # Pinned to an exact version
pkgs.go-bin.fromGoMod ./go.mod  # Auto-select from go.mod
```

`fromGoMod` reads the `toolchain` directive if present, otherwise resolves the latest patch for the `go` directive. For strict version matching that fails on mismatch, use `fromGoModStrict`.

## Go Tools

Pin Go ecosystem tools to your selected toolchain. Tools are accessed directly from the Go derivation:

```nix
let
  go = pkgs.go-bin.fromGoMod ./go.mod;
in {
  devShells.default = pkgs.mkShell {
    buildInputs = [
      # Option 1: curated defaults (delve, gopls, golangci-lint, ...)
      go.withDefaultTools

      # Option 2: pick your own
      (go.withTools [ "govulncheck" "gofumpt" ])

      # Option 3: individual access
      go.tools.govulncheck.latest
    ];
  };
}
```

Available tools: `delve`, `gofumpt`, `golangci-lint`, `gopls`, `govulncheck`, `staticcheck`. Incompatible versions throw with a clear error message suggesting the latest compatible version.

## Building a Go Application

### 1. Add govendor to your dev shell

```nix
devShells.default = pkgs.mkShell {
  buildInputs = [
    pkgs.go-bin.fromGoMod ./go.mod
    go-overlay.packages.${system}.govendor
  ];
};
```

### 2. Generate the manifest

```bash
govendor
```

This creates a `govendor.toml` with NAR hashes for all dependencies. Commit it to your repository. Re-run whenever dependencies change.

### 3. Build

```nix
pkgs.buildGoApplication {
  pname = "my-app";
  version = "1.0.0";
  src = ./.;
  go = pkgs.go-bin.fromGoMod ./go.mod;
  modules = ./govendor.toml;
}
```

That's it. No `vendorHash`, no patched Go binary.

> [!TIP]
> Use `govendor --check` in CI to detect manifest drift. See [Detecting Drift with Git Hooks](#detecting-drift-with-git-hooks) for automated checks.

## Builder Functions

go-overlay provides three builder functions. For full option tables, see [Reference](docs/reference.md).

### `buildGoApplication`

Build a single-module Go application. Supports [in-tree vendor](examples/vendor/) or [manifest mode](examples/http-server/). Handles [local replace directives](examples/library/), [cross-compilation](examples/cross-compile/), [build tags](examples/build-tags/), [testing](examples/http-server/), and [code generation](examples/codegen/).

```nix
pkgs.buildGoApplication {
  pname = "myapp";
  version = "1.0.0";
  src = ./.;
  go = pkgs.go-bin.fromGoMod ./go.mod;
  modules = ./govendor.toml;
  subPackages = [ "cmd/myapp" ];
  ldflags = [ "-s" "-w" ];
  doCheck = true;
}
```

### `buildGoWorkspace`

Build applications from a [Go workspace](https://go.dev/doc/tutorial/workspaces) (`go.work`). Each binary is built separately using the same manifest. See the [monorepo example](examples/monorepo/).

```nix
pkgs.buildGoWorkspace {
  pname = "api";
  version = "1.0.0";
  src = ./.;
  go = pkgs.go-bin.fromGoMod ./go.mod;
  modules = ./govendor.toml;
  subPackages = [ "api" ];
}
```

> [!NOTE]
> go-overlay uses the `[workspace]` section in `govendor.toml` to generate `go.work` during the build if one isn't present in the source tree. You don't need to commit `go.work` — the manifest is the single source of truth.

### `mkVendorEnv`

Lower-level function for custom build workflows. Creates a vendor directory from a manifest for use with `stdenv.mkDerivation`. See the [custom-build example](examples/custom-build/).

## Detecting Drift with Git Hooks

Use [cachix/git-hooks.nix](https://github.com/cachix/git-hooks.nix) to check for manifest drift on commit:

```nix
pre-commit-check = git-hooks.lib.${system}.run {
  src = ./.;
  hooks.govendor = {
    enable = true;
    name = "govendor";
    entry = "${go-overlay.packages.${system}.govendor}/bin/govendor --check";
    files = "(^|/)go\\.(mod|work)$";
    pass_filenames = true;
  };
};
```

## Private Modules

Pass a `.netrc` file into the build sandbox for authentication:

```nix
pkgs.buildGoApplication {
  pname = "myapp";
  version = "1.0.0";
  src = ./.;
  go = pkgs.go-bin.latest;
  modules = ./govendor.toml;
  netrcFile = "${builtins.getEnv "HOME"}/.netrc";
}
```

```bash
export GOPRIVATE="github.com/myorg/*"
nix build --impure
```

`builtins.getEnv` requires `--impure`. For pure builds, use [git-crypt](https://github.com/AGWA/git-crypt) or [sops-nix](https://github.com/Mic92/sops-nix) and reference the encrypted file as a relative path (`netrcFile = ./.netrc;`).

> [!CAUTION]
> The `.netrc` file is copied into the Nix store, which is world-readable by default.

## Using with buildGoModule

Override nixpkgs' Go toolchain when using `buildGoModule`:

```nix
(pkgs.buildGoModule.override { go = pkgs.go-bin.versions."1.22.3"; }) {
  pname = "my-app";
  version = "1.0.0";
  src = ./.;
  vendorHash = "sha256-...";
}
```

> [!WARNING]
> Passing `go` as a build argument does **not** work — you must use `.override`.

## Examples

The [examples/](examples/) directory contains self-contained Go projects demonstrating each feature. Every example can be built with `nix build .#example-<name>`.

| Example                                  | Features                                    |
| :--------------------------------------- | :------------------------------------------ |
| [hello](examples/hello/)                 | stdlib-only, no manifest                    |
| [http-server](examples/http-server/)     | External deps, `doCheck`                    |
| [cli](examples/cli/)                     | `ldflags`, `subPackages`, version injection |
| [cross-compile](examples/cross-compile/) | `GOOS`, `GOARCH`, `CGO_ENABLED`             |
| [build-tags](examples/build-tags/)       | `tags` parameter                            |
| [library](examples/library/)             | Local replace, `localReplaces`              |
| [codegen](examples/codegen/)             | Go 1.25 `tool` directive, `preBuild`        |
| [vendor](examples/vendor/)               | Committed `vendor/` directory               |
| [monorepo](examples/monorepo/)           | `buildGoWorkspace`, `go.work`               |
| [custom-build](examples/custom-build/)   | `mkVendorEnv` + `stdenv.mkDerivation`       |

## Documentation

- [reference.md](docs/reference.md) — Full option tables for all builder functions, library functions, and traditional Nix installation.
- [govendor-toml-v2.md](docs/govendor-toml-v2.md) — `govendor.toml` v2 reference.
- [migrating.md](docs/migrating.md) — Migration guides from gomod2nix and buildGoModule.

## Used By

- [devenv](https://github.com/cachix/devenv) — Fast, declarative, reproducible developer environments.

---

<sub>The go-overlay logo was generated using Google Gemini. The Go gopher was originally designed by <a href="https://reneefrench.blogspot.com/">Renee French</a> and is licensed under <a href="https://creativecommons.org/licenses/by/4.0/">CC BY 4.0</a>. The <a href="https://github.com/NixOS/nixos-artwork">Nix snowflake</a> is a trademark of the NixOS Foundation. This project is not affiliated with the Go project, the NixOS Foundation, or Google.</sub>
