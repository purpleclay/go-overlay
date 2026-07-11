# How go-overlay Works

go-overlay builds Go applications with an **off-the-shelf Go toolchain** inside the Nix sandbox — no patches, no forks. This document explains why that constraint shapes the whole design, how `govendor` resolves dependencies, and what the manifest buys you at build time.

## The problem

The Nix build sandbox has no network access, but `go build` expects to fetch modules from a proxy. Every Nix-and-Go integration is an answer to this mismatch, and the answers fall into two families:

1. **Patch the toolchain.** nixpkgs carries a `go_no_vendor_checks` patch that adds an escape hatch (`GO_NO_VENDOR_CHECKS=1`) to `cmd/go`, relaxing the consistency checks Go performs on vendor directories. With the checks disabled, a builder can symlink module sources into `vendor/` and skip producing a `vendor/modules.txt` entirely. This is how gomod2nix works — its lean manifest is possible _because_ the patched toolchain never asks for package-level detail.
2. **Satisfy the toolchain.** Produce a `vendor/` directory that stock Go accepts: complete, consistent, with a `modules.txt` that passes every check `go build -mod=vendor` performs. This is go-overlay's approach.

The second path is harder — a valid `modules.txt` must list every vendored package, mark explicit requirements, and record each module's Go version — but it keeps a property the first path gives up: **the Go that builds your code is the Go that Google ships.** The vendor-checks patch tracks a moving target; new Go releases add checks the patch must also disable, and each Go upgrade carries a re-patching risk that lands on someone else's schedule. An unpatched toolchain has no such coupling — when a new Go version is released, go-overlay picks it up within hours because there is nothing to adapt.

## Why `modules.txt` fidelity matters

Skipping `modules.txt` isn't just a compliance shortcut — it discards information the compiler uses.

Each module's annotation line in `modules.txt` (`## explicit; go 1.21`) records the Go version that module declares. In vendor mode, stock Go reads these annotations to select the **language version each dependency is compiled with**. This has been observable behaviour since Go 1.22, which changed loop variable scoping: a dependency written against Go 1.22 semantics must be compiled as Go 1.22 code. When no `modules.txt` exists, that information is gone, and dependencies fall back to a conservative default language version — a subtle semantics change that no error message will ever point at.

govendor records each module's declared Go version in the manifest (`go = "1.21"`) and whether the module lacks Go's `## explicit` annotation (`implicit = true`), and the builder writes both back into the `modules.txt` it assembles. Stock Go then compiles every dependency exactly as its author intended.

## How resolution works

A useful fact anchors the design: **in Go modules, version selection is platform-independent.** Minimal version selection over `go.mod` files never consults `GOOS` or `GOARCH`. Only _package attribution_ — which packages within each module are actually needed — varies with build constraints. govendor's pipeline separates the two:

1. **`go mod download -json`** — one invocation. Resolves and downloads every module in the build graph, and reports where each landed in the module cache.
2. **`go mod vendor -o <tmp>`** — one invocation. This is the platform-independence trick: `go mod vendor` performs package attribution with _every build constraint treated as satisfied_ (`imports.AnyTags` in `cmd/go`). One pass covers every `GOOS`/`GOARCH` pair Go recognises and every custom build tag, simultaneously — a strict superset of any finite platform list. Packages guarded by `//go:build windows`, `//go:build js && wasm`, or `//go:build yourcustomtag` are all attributed, unconditionally. The same pass covers `tool` directives, since Go includes tool dependencies in the pattern it vendors.
3. **Parse, don't emulate.** govendor reads the `modules.txt` that Go just wrote — module set, versions, package lists, explicit markers, Go versions, replace topology — then discards the temporary vendor directory. The manifest's structure is _derived from Go's own output_, not reconstructed by reimplementing Go's loader. Whatever `go build -mod=vendor` expects to find is, by construction, what the manifest describes.
4. **Enrich with content addresses.** Each module directory reported by step 1 is hashed into a NAR hash (SRI format) — the fixed-output derivation hash Nix uses to fetch it at build time. Hashes are reused from the previous manifest for any module whose version is unchanged: a module version's content is immutable, so re-hashing it is wasted work. Only new or upgraded modules are hashed. Local `replace` targets are the exception — their content is mutable, so they are hashed on every run.

The resulting `govendor.toml` is, in essence, **`modules.txt` enriched with content addresses**. There is no platform dimension anywhere in the pipeline, which is why schema v4 has no platform configuration: there is nothing left to configure.

### Workspaces

Workspace projects follow the same shape from the workspace root: workspace-level version selection, then a single `go work vendor` pass for attribution across all members, with member modules recorded as local, non-fetched entries whose hashes are retained for drift detection. Tool directives from member `go.mod` files are aggregated into one `[tool]` table.

## How building works

At build time the transformation runs in reverse. `buildGoApplication` and `buildGoWorkspace`:

1. Fetch each remote module as a fixed-output derivation — `go mod download` inside the sandbox, verified against the manifest's NAR hash. Each module is its own derivation, so a dependency bump refetches one module, not the world.
2. Assemble the vendor environment: module sources linked into `vendor/`, local replacements copied from the source tree, and `modules.txt` reconstituted from the manifest — package lists, explicit markers, and Go versions intact.
3. Compile any declared tools for the build host and inject them into `nativeBuildInputs`.
4. Build with `-mod=vendor`, `GOPROXY=off`, on an unpatched toolchain. Every consistency check Go performs, passes — because the file it validates is the file Go itself produced during resolution.

Cross-compilation needs no ceremony: the manifest already attributes packages for every platform, so overriding `GOOS`/`GOARCH` on the builder is the entire workflow.

## Performance characteristics

- **Cold generation** is dominated by `go mod download` (network) and NAR hashing. Package attribution is a single `go mod vendor` pass regardless of how many platforms you will ever target.
- **Warm regeneration** — the everyday case — reuses hashes for unchanged module versions. Bumping one dependency costs one download, one hash, and one attribution pass.
- **`govendor --check`** performs the same resolution and compares the result byte-for-byte against the committed manifest, so drift detection inherits the same warm-run economics. This keeps it fast enough for a pre-commit hook.
- The temporary vendor copy written by `go mod vendor -o` is the price of single-pass attribution; it lands in `$TMPDIR` (tmpfs on most NixOS systems) and is deleted immediately after `modules.txt` is parsed.
