# Reference

Full option tables and API documentation for go-overlay.

- [Selecting a Go Version](#selecting-a-go-version)
- [Go Tools](#go-tools)
  - [`withDefaultTools`](#withdefaulttools)
  - [`withTools`](#withtools)
  - [Individual Tool Access](#individual-tool-access)
- [Builder Functions](#builder-functions)
  - [`buildGoApplication`](#buildgoapplication)
  - [`buildGoWorkspace`](#buildgoworkspace)
  - [`mkVendorEnv`](#mkvendorenv)
- [Cross-Compilation](#cross-compilation)
- [Traditional Nix Installation](#traditional-nix-installation)

## Selecting a Go Version

### `go-bin.latest`

The absolute latest version, including release candidates.

### `go-bin.latestStable`

The latest stable version, excluding release candidates.

### `go-bin.versions.<version>`

Pin to an exact version. Example: `go-bin.versions."1.22.3"`.

### `go-bin.hasVersion <version>`

Returns `true` if the version exists in the overlay.

```nix
if go-bin.hasVersion "1.22.0"
then go-bin.versions."1.22.0"
else go-bin.latestStable
```

### `go-bin.isDeprecated <version>`

Returns `true` if the version is EOL according to [Go's release policy](https://go.dev/doc/devel/release#policy). Go supports the current and previous minor versions.

### `go-bin.fromGoMod <path>`

Auto-select Go version from `go.mod`. Uses the `toolchain` directive if present, otherwise resolves the latest patch of the `go` directive.

### `go-bin.fromGoModStrict <path>`

Strict version matching from `go.mod`. No automatic patch version selection; fails if the exact version is unavailable.

| go.mod Declaration               | `fromGoMod`   | `fromGoModStrict` |
| :------------------------------- | :------------ | :---------------- |
| `go 1.21`                        | Latest 1.21.x | Error             |
| `go 1.21.6`                      | 1.21.6        | 1.21.6            |
| `go 1.21` + `toolchain go1.21.6` | 1.21.6        | 1.21.6            |

## Go Tools

Tools are accessed from a Go toolchain derivation. Three access patterns are available:

### `withDefaultTools`

Bundles the toolchain with a curated set of essential tools:

| Tool            | Role                  |
| :-------------- | :-------------------- |
| `delve`         | Debugger              |
| `gofumpt`       | Formatter             |
| `golangci-lint` | Linter                |
| `gopls`         | Language server       |
| `govulncheck`   | Vulnerability scanner |
| `staticcheck`   | Static analysis       |

### `withTools`

Select specific tools by name or pin versions:

```nix
go.withTools [
  "govulncheck"                              # latest compatible
  { name = "gofumpt"; version = "0.7.0"; }   # pinned version
]
```

### Individual Tool Access

```nix
go.tools.govulncheck.latest      # latest compatible
go.tools.govulncheck."1.1.3"     # pinned version
```

Incompatible versions evaluate to a `throw` with a clear error message:

```text
go-overlay: govulncheck 1.1.4 requires Go >= 1.22.0,
but the selected toolchain is Go 1.21.4.

Latest compatible version: 1.1.3
```

## Builder Functions

### `buildGoApplication`

Build a single-module Go application using vendored dependencies.

| Option             | Default             | Description                                                |
| :----------------- | :------------------ | :--------------------------------------------------------- |
| `pname`            | required            | Package name                                               |
| `version`          | required            | Package version                                            |
| `src`              | required            | Source directory                                           |
| `go`               | required            | Go derivation from go-overlay                              |
| `modules`          | `null`              | Path to govendor.toml manifest (null = use in-tree vendor) |
| `subPackages`      | `["."]`             | Packages to build (relative to src)                        |
| `ldflags`          | `[]`                | Linker flags                                               |
| `tags`             | `[]`                | Build tags                                                 |
| `allowGoReference` | `false`             | Allow Go toolchain in runtime closure                      |
| `localReplaces`    | `{}`                | Map of module path to Nix path for external local replaces |
| `netrcFile`        | `null`              | Path to a `.netrc` file for private module authentication  |
| `GOPRIVATE`        | `""`                | Module path prefixes to bypass the proxy and checksum DB   |
| `GONOSUMDB`        | `""`                | Module path prefixes to bypass the checksum DB only        |
| `GONOPROXY`        | `""`                | Module path prefixes to bypass the proxy only              |
| `doCheck`          | `false`             | Run tests during the build                                 |
| `checkFlags`       | `[]`                | Additional flags passed to `go test`                       |
| `excludedPackages` | `[]`                | Packages to exclude from testing                           |
| `CGO_ENABLED`      | inherited from `go` | Enable CGO                                                 |
| `GOOS`             | inherited from `go` | Target operating system                                    |
| `GOARCH`           | inherited from `go` | Target architecture                                        |

### `buildGoWorkspace`

Build applications from a Go workspace (`go.work`).

| Option             | Default             | Description                                                |
| :----------------- | :------------------ | :--------------------------------------------------------- |
| `pname`            | required            | Package name                                               |
| `version`          | required            | Package version                                            |
| `src`              | required            | Source directory (workspace root)                          |
| `go`               | required            | Go derivation from go-overlay                              |
| `modules`          | `null`              | Path to govendor.toml manifest (null = use in-tree vendor) |
| `subPackages`      | `["."]`             | Packages to build (relative to workspace root)             |
| `ldflags`          | `[]`                | Linker flags                                               |
| `tags`             | `[]`                | Build tags                                                 |
| `allowGoReference` | `false`             | Allow Go toolchain in runtime closure                      |
| `netrcFile`        | `null`              | Path to a `.netrc` file for private module authentication  |
| `GOPRIVATE`        | `""`                | Module path prefixes to bypass the proxy and checksum DB   |
| `GONOSUMDB`        | `""`                | Module path prefixes to bypass the checksum DB only        |
| `GONOPROXY`        | `""`                | Module path prefixes to bypass the proxy only              |
| `doCheck`          | `false`             | Run tests during the build                                 |
| `checkFlags`       | `[]`                | Additional flags passed to `go test`                       |
| `excludedPackages` | `[]`                | Packages to exclude from testing                           |
| `CGO_ENABLED`      | inherited from `go` | Enable CGO                                                 |
| `GOOS`             | inherited from `go` | Target operating system                                    |
| `GOARCH`           | inherited from `go` | Target architecture                                        |

### `mkVendorEnv`

Create a vendor directory with `modules.txt` from a parsed `govendor.toml` manifest. This is the lower-level function used internally by `buildGoApplication`.

| Option          | Default  | Description                                                |
| :-------------- | :------- | :--------------------------------------------------------- |
| `go`            | required | Go derivation from go-overlay                              |
| `manifest`      | required | Parsed govendor.toml (via `builtins.fromTOML`)             |
| `src`           | `null`   | Source tree (required if manifest has local modules)       |
| `localReplaces` | `{}`     | Map of module path to Nix path for external local replaces |
| `netrcFile`     | `null`   | Path to a `.netrc` file for private module authentication  |
| `GOPRIVATE`     | `""`     | Module path prefixes to bypass the proxy and checksum DB   |
| `GONOSUMDB`     | `""`     | Module path prefixes to bypass the checksum DB only        |
| `GONOPROXY`     | `""`     | Module path prefixes to bypass the proxy only              |

## Cross-Compilation

Build binaries for different platforms by overriding `GOOS` and `GOARCH`. Works with both `buildGoApplication` and `buildGoWorkspace`.

By default, `govendor` resolves dependencies for these platforms:

- `linux/amd64`, `linux/arm64`
- `darwin/amd64`, `darwin/arm64`
- `windows/amd64`, `windows/arm64`

To build for platforms outside the defaults, use `--include-platform` when generating the manifest:

```bash
govendor --include-platform=freebsd/amd64
```

The additional platforms are persisted in `govendor.toml` and automatically used on subsequent runs. See the [cross-compile example](../examples/cross-compile/).

## Traditional Nix Installation

For users not using flakes, go-overlay can be imported directly as an overlay.

> [!TIP]
> For reproducible builds, pin to a specific commit instead of `main`:
>
> ```nix
> builtins.fetchTarball "https://github.com/purpleclay/go-overlay/archive/<commit-sha>.tar.gz"
> ```
>
> Find commit SHAs in the [go-overlay commit history](https://github.com/purpleclay/go-overlay/commits/main).

### Using fetchTarball

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

### User Overlays

Add to `~/.config/nixpkgs/overlays.nix`:

```nix
[
  (import (builtins.fetchTarball
    "https://github.com/purpleclay/go-overlay/archive/main.tar.gz"))
]
```

### Nix Channels

```bash
nix-channel --add https://github.com/purpleclay/go-overlay/archive/main.tar.gz go-overlay
nix-channel --update
```

```nix
let
  go-overlay = import <go-overlay>;
  pkgs = import <nixpkgs> { overlays = [ go-overlay ]; };
in
pkgs.go-bin.latest
```
