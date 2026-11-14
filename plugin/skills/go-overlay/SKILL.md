---
name: go-overlay
description: Use when a flake imports go-overlay, a govendor.toml is present, Go dependencies or imports change in such a project, or a Nix build of a Go project fails with a vendoring, hash or Go-version error. Covers the purpleclay go-overlay Nix overlay (buildGoApplication, buildGoWorkspace, go-bin toolchains, govendor, govendor.toml).
---

# go-overlay

go-overlay builds Go projects in Nix from a generated manifest, `govendor.toml`. The manifest is Go's `vendor/modules.txt` plus one NAR hash per module. The builders recreate a vendor tree from it and compile with `-mod=vendor` and no network.

Nearly every failure is the manifest falling behind the source. Keep the two in step and little else goes wrong.

## The loop

Run this after any change to `go.mod`, `go.sum`, `go.work`, **or to which packages the code imports**:

1. Update `go.mod` with Go itself: `go get example.com/mod@version` for a new dependency, then `go mod tidy`.
2. `git add` new files. Flakes and `govendor` only see tracked files.
3. `govendor`, run from the directory that holds the manifest. For a workspace that is the workspace root.
4. `govendor --check` must exit `0`.
5. `git add govendor.toml`. The run in step 3 changed it after step 2.
6. `nix build` must exit `0`. Do not pipe it through `tail` or `grep` when you need the exit code.
7. Commit `govendor.toml` in the same commit as the change.

If the build uses `buildGoVendoredApplication` or `buildGoVendoredWorkspace`, there is no `govendor.toml`. Replace steps 3 to 5 with `go mod vendor` (or `go work vendor` for a workspace) and `git add vendor/`, and commit `vendor/` with the change.

Run `govendor` from the project's dev shell (`nix develop --command govendor`). If the dev shell does not provide it, use the version the flake pins: `nix run --inputs-from . go-overlay#govendor`. Do not run an unpinned `github:purpleclay/go-overlay#govendor`, which can write a newer schema than the pinned builder reads.

A change only to Nix files, such as `default.nix` or `flake.nix`, does not need the loop. `nix build` is enough.

The import rule is the one people miss. Importing `golang.org/x/sync/semaphore` when only `errgroup` was imported changes neither `go.mod` nor `go.sum`, but the manifest's package list is now stale and the Nix build fails. Regenerate.

Exit codes: `0` up to date or generated, `1` drift or missing manifest, `2` execution error.

Never do these:

- Edit `govendor.toml` by hand. It is generated.
- Add `vendorHash`, `buildGoModule` or gomod2nix files. go-overlay replaces them.
- Commit a `vendor/` directory, unless the project already uses `buildGoVendoredApplication` or `buildGoVendoredWorkspace`.

## Setting up a project

Apply the overlay, then pick the toolchain from `go.mod`:

```nix
pkgs = import nixpkgs {
  inherit system;
  overlays = [ go-overlay.overlays.default ];
};
go = pkgs.go-bin.fromGoMod ./go.mod;
```

Use `fromGoMod`. `go-bin.latest` can be a release candidate; use `go-bin.latestStable` if the project must float.

| Project shape                   | Builder                      | Manifest                                  |
| ------------------------------- | ---------------------------- | ----------------------------------------- |
| One module                      | `buildGoApplication`         | `govendor.toml` beside `go.mod`           |
| `go.work` workspace             | `buildGoWorkspace`           | One `govendor.toml` at the workspace root |
| One module, committed `vendor/` | `buildGoVendoredApplication` | None                                      |
| Workspace, committed `vendor/`  | `buildGoVendoredWorkspace`   | None                                      |

```nix
pkgs.buildGoApplication {
  inherit go;
  pname = "example";
  version = "0.1.0";
  src = ./.;
  meta.mainProgram = "example";
}
```

A project with no dependencies still needs a manifest. Run `govendor`; it writes one with an empty `[mod]` table.

Dev shell: add `go-overlay.packages.${system}.govendor` and the toolchain. Tools come from the toolchain: `go.withDefaultTools`, or `go.withTools [ "gopls" { name = "gofumpt"; version = "0.7.0"; } ]`.

## Rules that prevent quiet mistakes

- **Flags.** Add Go flags with `extraGoFlags`. Do not set `env.GOFLAGS` or `env.GODEBUG`: each replaces a value the builder computes, including `-mod=vendor`.
- **Test exclusions.** `excludedPackages` takes full import paths: the `module` line of `go.mod` plus the directory, such as `github.com/acme/app/internal/e2e`. A relative path such as `internal/e2e` excludes nothing.
- **Local replaces.** A `replace` that points inside `src` needs no extra option. `localReplaces` is only for a path outside `src`.
- **Cross-compiling.** Set `GOOS` and `GOARCH` on the builder. The manifest already covers every platform; do not regenerate it.
- **`tool` directives.** Tools named in `go.mod` are built for the build machine automatically. A tool whose package lives in the project's own module is not supported.
- **Private modules.** Set `GOPRIVATE = "github.com/acme"` on the builder, and `netrcFile` for credentials. The builder reads the netrc with `builtins.readFile`, so the token is copied into the world-readable Nix store. Tell the user before adding it. In a flake, a path outside the repo fails pure evaluation. Either the user builds with `--impure` and reads the path from an environment variable, or the netrc comes from a secrets tool. Never commit a netrc. `govendor` itself uses the user's normal Go setup, so `GOPRIVATE` and `~/.netrc` must work for `go mod download` there too.
- **Pinning.** Pin the `go-overlay` input to a tag or commit, not `main`.

## Workspaces

- Generate from the workspace root: `govendor` there, not inside a member.
- The root manifest is the one the workspace build reads. If members also have their own `govendor.toml`, keep those current too, or a `govendor --check` pre-commit hook fails on the member's `go.mod`. Run both from the root: `govendor` for the workspace manifest, then `govendor --recursive` for the member manifests. `--recursive` ignores `go.work`, so it never updates the root manifest. Check with `govendor --check` and `govendor --check --recursive`.
- Commit `go.work`. Both `govendor` and the build are most reliable with it present.
- Select what to build with `subPackages = [ "api" ];`.

## When a build fails

| Message contains                                      | Cause                                                                                                                                                                  | Fix                                                                                      |
| ----------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------- |
| `cannot find module providing package`                | The manifest's package list is stale                                                                                                                                   | Run the loop                                                                             |
| `inconsistent vendoring`                              | `go.mod` or `go.work` changed after the manifest was written                                                                                                           | Run the loop                                                                             |
| `hash mismatch in fixed-output derivation`            | A wrong hash in `govendor.toml`, often from a bad merge. `govendor` reuses existing hashes, so re-running it will not repair one, and `govendor --check` still passes. | Delete `govendor.toml`, run `govendor`, commit. Do not paste the `got:` hash in by hand. |
| `Could not parse Go version`                          | The `go` line in `go.mod` has a trailing comment or unusual spacing                                                                                                    | Make it a bare `go 1.x.y` line                                                           |
| `Go version '…' is not available`                     | The overlay input predates that release                                                                                                                                | `nix flake update go-overlay`                                                            |
| `is not inside a git repository`                      | A local module is not in git                                                                                                                                           | `git init`, `git add`, then run the loop                                                 |
| `uses schema v…, current govendor requires schema v…` | The manifest predates the installed govendor                                                                                                                           | Run `govendor` and commit                                                                |

## Before you finish

`govendor --check` exits `0` (manifest builds only), `nix build` succeeds, and `govendor.toml` or `vendor/` is staged with the change that needed it.
