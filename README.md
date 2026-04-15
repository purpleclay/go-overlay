<div align="center">
  <img src="https://github.com/purpleclay/go-overlay/raw/main/docs/static/go-overlay.png" alt="go-overlay logo" width="220px" />
  <h1>go-overlay</h1>
  <p>A complete Go development environment for Nix.<br>Toolchains, tools, and builders — pure, reproducible, and auto-updated.</p>
  <img alt="Nix" src="https://img.shields.io/badge/Nix-5277C3?logo=nixos&logoColor=white" />
  <img alt="Go" src="https://img.shields.io/badge/Go-00ADD8?logo=go&logoColor=white" />
  <a href="LICENSE"><img alt="MIT" src="https://img.shields.io/badge/MIT-gray?logo=github&logoColor=white" /></a>
  <a href="https://github.com/purpleclay/go-overlay/actions/workflows/go-update.yml"><img alt="Go Update" src="https://github.com/purpleclay/go-overlay/actions/workflows/go-update.yml/badge.svg" /></a>
</div>
<br>

- **100+ Go versions** — from 1.17 to latest, including release candidates, <u>updated within 4 hours</u> of a new release on [go.dev](https://go.dev/dl/).
- **No stale `vendorHash`** — dependencies are pinned per-module with NAR hashes. Change a dep, re-run `govendor`. No hash archaeology.
- **Workspace support** — build multi-module `go.work` monorepos reproducibly. Neither `buildGoModule` nor gomod2nix can do this.
- **Go tools pinned to your toolchain** — govulncheck, gopls, golangci-lint, and more, <u>updated within 6 hours</u> of release and version-locked to your selected Go version with a clear error if incompatible.
- **Private modules** — standard Go authentication via `.netrc`, no custom infrastructure required.

## Getting Started

### Try Go without installing anything

```bash
nix run github:purpleclay/go-overlay -- version
# go version go1.26.2 linux/amd64
```

### Create a new project

The template bootstraps a new project with a dev shell, builder, and drift detection pre-configured:

```bash
nix flake new -t github:purpleclay/go-overlay my-app
cd my-app && nix develop
```

### Onboard an existing project

#### 1. Add the flake input

Add go-overlay to your `flake.nix` inputs and apply the overlay:

```nix
{
  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
    go-overlay.url = "github:purpleclay/go-overlay";
  };

  outputs = { nixpkgs, flake-utils, go-overlay, ... }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs {
          inherit system;
          overlays = [ go-overlay.overlays.default ];
        };
      in { ... }
    );
}
```

> [!TIP]
> Not using flakes? See the [traditional Nix installation guide](docs/reference.md#traditional-nix-installation).

#### 2. Add govendor to your dev shell

`govendor` generates and maintains the dependency manifest used during builds:

```nix
devShells.default = pkgs.mkShell {
  buildInputs = [
    go-overlay.packages.${system}.govendor
  ];
};
```

#### 3. Generate and commit the manifest

```bash
govendor
```

This creates a `govendor.toml` with NAR hashes for all dependencies. Commit it to your repository.

> [!IMPORTANT]
> Re-run `govendor` whenever you add, remove, or upgrade a dependency. Use `govendor --check` in CI to catch drift before it reaches production.

#### 4. Build

Create a `default.nix` file to define your package:

```nix
{ pkgs, go }:
pkgs.buildGoApplication {
  inherit go;
  pname = "my-app";
  version = "0.1.0";
  src = ./.;
  modules = ./govendor.toml;
}
```

Then wire it into your `flake.nix` outputs:

```nix
let
  go = pkgs.go-bin.fromGoMod ./go.mod;
in {
  packages.default = pkgs.callPackage ./default.nix { inherit go; };
}
```

> [!TIP]
> `fromGoMod` auto-selects the Go version from your `go.mod`. For other version selection options, see the [reference guide](docs/reference.md#selecting-a-go-version).

And then build it:

```bash
nix build
```

That's it. No stale `vendorHash` to fix after every dependency change. 👋

## Examples

Not sure how to configure a specific feature? Each example is a self-contained, buildable project — find the pattern you need and use it as a starting point.

| Example                                                      | Features                                               |
| :----------------------------------------------------------- | :----------------------------------------------------- |
| [hello-world](examples/hello-world/)                         | stdlib-only, no manifest                               |
| [http-chi-server](examples/http-chi-server/)                 | External deps, `modules`                               |
| [cobra-cli](examples/cobra-cli/)                             | `ldflags`, `subPackages`, `doCheck`, version injection |
| [cross-compile](examples/cross-compile/)                     | `GOOS`, `GOARCH`, `CGO_ENABLED`                        |
| [build-tags](examples/build-tags/)                           | `tags` parameter                                       |
| [local-replaces](examples/local-replaces/)                   | Local replace directives, `localReplaces`              |
| [oapi-codegen](examples/oapi-codegen/)                       | `tool` directive, `preBuild` code generation           |
| [sqlc-codegen](examples/sqlc-codegen/)                       | `nativeBuildInputs`, `preBuild` code generation        |
| [vendored](examples/vendored/)                               | Committed `vendor/` directory                          |
| [go-workspace](examples/go-workspace/)                       | `buildGoWorkspace`, `go.work`, `subPackages`           |
| [go-workspace-inferred](examples/go-workspace-inferred/)     | `buildGoWorkspace`, inferred `go.work`                 |
| [go-workspace-vendored](examples/go-workspace-vendored/)     | `buildGoWorkspace`, committed `vendor/`                |
| [wasm-build](examples/wasm-build/)                           | `mkVendorEnv` + `stdenv.mkDerivation`                  |
| [nixpkgs-build-go-module](examples/nixpkgs-build-go-module/) | `buildGoModule.override` with go-overlay toolchain     |

Build or run any example directly:

```bash
# pattern: nix build .#example-<name>
nix build .#example-cobra-cli
nix run .#example-cobra-cli
```

## Go Tools

Every Go toolchain derivation includes version-locked tools — `gopls`, `golangci-lint`, `govulncheck`, `delve`, and more — pinned to your selected Go version and updated within 6 hours of a new release.

Add them all to your dev shell with a single attribute:

```nix
let
  go = pkgs.go-bin.fromGoMod ./go.mod;
in {
  devShells.default = pkgs.mkShell {
    buildInputs = [
      go.withDefaultTools
    ];
  };
}
```

For selecting specific tools or pinning versions, see the [Go Tools reference](docs/reference.md#go-tools).

## Detecting Drift with Git Hooks

Use [cachix/git-hooks.nix](https://github.com/cachix/git-hooks.nix) to check for manifest drift on commit:

```nix
let
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
in {
  devShells.default = pkgs.mkShell {
    inherit (pre-commit-check) shellHook;
    buildInputs = pre-commit-check.enabledPackages;
  };
}
```

## Private Go Modules

Private modules require two things: bypassing the public proxy, and credentials to authenticate with the private host. Set `GOPRIVATE` to route around the proxy and `netrcFile` to provide credentials:

> [!TIP]
> `GOPRIVATE` implicitly sets `GONOPROXY` and `GONOSUMDB` — you only need to set all three explicitly if you require different values for each.

```nix
{ pkgs, go }:
pkgs.buildGoApplication {
  inherit go;
  pname = "myapp";
  version = "1.0.0";
  src = ./.;
  modules = ./govendor.toml;
  netrcFile = "${builtins.getEnv "HOME"}/.netrc";
  GOPRIVATE = "<your-private-host>/*";
}
```

> [!NOTE]
> `builtins.getEnv "HOME"` reads the host environment to locate `~/.netrc` — this is why `--impure` is required. The file contents are read at eval time and passed into the build sandbox. Credentials are not stored in the repo. If you prefer to keep a `.netrc` inside the source root, consider encrypting it with [git-crypt](https://github.com/AGWA/git-crypt) or [sops-nix](https://github.com/Mic92/sops-nix).

```bash
nix build --impure
```

> [!CAUTION]
> The `.netrc` contents are embedded in the derivation, which is stored in the Nix store. The Nix store is world-readable by default.

## Further Reading

- [reference.md](docs/reference.md) — Full option tables for all builder functions, library functions, and traditional Nix installation.
- [govendor-toml-v2.md](docs/govendor-toml-v2.md) — `govendor.toml` schema reference.
- [migrating.md](docs/migrating.md) — Migration guides from gomod2nix and buildGoModule.

---

<sub>The go-overlay logo was generated using Google Gemini. The Go gopher was originally designed by <a href="https://reneefrench.blogspot.com/">Renee French</a> and is licensed under <a href="https://creativecommons.org/licenses/by/4.0/">CC BY 4.0</a>. The <a href="https://github.com/NixOS/nixos-artwork">Nix snowflake</a> is a trademark of the NixOS Foundation. This project is not affiliated with the Go project, the NixOS Foundation, or Google.</sub>
