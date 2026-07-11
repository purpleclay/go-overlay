# Migration Guides

## From gomod2nix

### Before

```nix
{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
    gomod2nix.url = "github:nix-community/gomod2nix";
  };

  outputs = { nixpkgs, flake-utils, gomod2nix, ... }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs {
          inherit system;
          overlays = [ gomod2nix.overlays.default ];
        };
      in {
        packages.default = pkgs.buildGoApplication {
          pname = "myapp";
          version = "1.0.0";
          src = ./.;
          modules = ./gomod2nix.toml;
        };
      }
    );
}
```

```bash
gomod2nix generate
```

### After

```nix
{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
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
      in {
        packages.default = pkgs.buildGoApplication {
          pname = "myapp";
          version = "1.0.0";
          src = ./.;
          go = pkgs.go-bin.fromGoMod ./go.mod;
          modules = ./govendor.toml;
        };
      }
    );
}
```

```bash
govendor
```

### Steps

1. Replace `gomod2nix` with `go-overlay` in flake inputs
2. Update the overlay reference
3. Add `go` parameter to `buildGoApplication`
4. Run `govendor` to generate the new manifest
5. Delete `gomod2nix.toml` and commit `govendor.toml`

## From buildGoModule

### Before

```nix
{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { nixpkgs, flake-utils, ... }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in {
        packages.default = pkgs.buildGoModule {
          pname = "myapp";
          version = "1.0.0";
          src = ./.;
          vendorHash = "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=";
        };
      }
    );
}
```

### After

```nix
{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
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
      in {
        packages.default = pkgs.buildGoApplication {
          pname = "myapp";
          version = "1.0.0";
          src = ./.;
          go = pkgs.go-bin.fromGoMod ./go.mod;
          modules = ./govendor.toml;
        };
      }
    );
}
```

```bash
govendor
```

### Steps

1. Add `go-overlay` to flake inputs
2. Add the overlay to `pkgs`
3. Replace `buildGoModule` with `buildGoApplication`
4. Remove `vendorHash`
5. Add `go` and `modules` parameters
6. Run `govendor` to generate the manifest
7. Commit `govendor.toml`

## From schema v3 to v4

Schema v4 accompanies a rework of how `govendor` resolves dependencies: package attribution now comes from a single platform-independent `go mod vendor` pass instead of per-platform `go list` invocations. Every `GOOS`/`GOARCH` pair and build tag is covered unconditionally, so platform configuration no longer exists. See [How go-overlay Works](how-it-works.md) for the full model.

### What changed

- `include_platforms` is removed from the manifest, and the `--include-platform` flag is removed from the CLI. Passing the flag is now an execution error (exit code `2`) rather than a silent no-op, so automation still passing it fails loudly.
- `[mod]` entries gain an optional `implicit` field, preserving Go's own `modules.txt` annotations.
- The `[mod]` table now contains only modules that vendor packages or are structurally required — expect the manifest to shrink, and builds to fetch fewer modules.

### Steps

1. Upgrade go-overlay to v2.0.0 or later
2. Remove `--include-platform` from any scripts, git hooks, or CI invocations
3. Run `govendor` to regenerate — NAR hashes for unchanged module versions are carried forward, so this is fast
4. Commit the regenerated `govendor.toml`

### What CI shows before you migrate

`govendor --check` against a v3 manifest reports a schema mismatch and exits `1`, the same exit code as drift. The fix is the same in both cases: regenerate and commit. No build will silently consume a stale-schema manifest — `buildGoApplication` and `buildGoWorkspace` reject manifests with an unexpected `schema` value.

> [!NOTE]
> Projects that used `--include-platform` for targets like `freebsd/amd64` or `js/wasm` need no
> replacement: those packages are attributed automatically under v4. The [cross-compile](../examples/cross-compile/)
> and [wasm-build](../examples/wasm-build/) examples show the simplified workflow.
