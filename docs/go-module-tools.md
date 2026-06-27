# Go Module Tool Directives

Go 1.24 introduced `tool` directives in `go.mod` as a first-class way to pin code-generation and other build tools alongside your module dependencies:

```go.mod
tool github.com/a-h/templ/cmd/templ
tool github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen
```

go-overlay treats these as **host-platform build inputs**. When `govendor` generates a manifest, tool directives are recorded in a dedicated `[tool]` section. Both `buildGoApplication` and `buildGoWorkspace` read this section and compile each tool for the host platform, injecting the binaries into `nativeBuildInputs` automatically.

Tool binaries are available in `$PATH` during `preBuild` without any extra configuration:

```nix
pkgs.buildGoApplication {
  inherit go;
  pname = "myapp";
  version = "1.0.0";
  src = ./.;
  modules = ./govendor.toml;

  preBuild = ''
    templ generate ./...
  '';
}
```

No `nativeBuildInputs = [ templ ];` required — the builder handles it.

For workspace projects (`go.work`), tool directives are aggregated across all member `go.mod` files into a single `[tool]` section.

`govendor --check` validates the `[tool]` section against `go.mod`, so adding or removing a tool directive is treated as drift and triggers regeneration.

## `tool` directive vs `nativeBuildInputs`

Not every build-time tool belongs in `go.mod`. The [oapi-codegen](../examples/oapi-codegen/) and [sqlc-codegen](../examples/sqlc-codegen/) examples demonstrate the two approaches side by side — the former via a `tool` directive, the latter via `nativeBuildInputs`.

|                          | `tool` directive                                                                                                                                                                                                                                                                                                                                                                                          | `nativeBuildInputs`                                                                                 |
| :----------------------- | :-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | :-------------------------------------------------------------------------------------------------- |
| Source of truth          | `go.mod` tool directive                                                                                                                                                                                                                                                                                                                                                                                   | flake / overlay pin                                                                                 |
| Version                  | travels with the module's dependency graph                                                                                                                                                                                                                                                                                                                                                                | independent; can drift from `go.mod`                                                                |
| Manifest generation cost | resolved by `govendor` alongside your module's own dependencies. The tool itself only ever builds for the host platform, but `govendor` currently re-walks its dependency graph once for every entry in your target platform list, even though that graph doesn't vary by target — for tools with large transitive graphs (`sqlc`, `oapi-codegen`), this redundancy adds real time to `govendor generate` | no added cost; the tool is resolved independently of `govendor.toml`                                |
| Build caching            | project-local; decoupled from your application source, so the host tool only rebuilds when `go.mod`/`go.sum` change                                                                                                                                                                                                                                                                                       | content-addressed on the tool itself, shareable across projects via a binary cache                  |
| Packaging effort         | none — just add `tool` to `go.mod`                                                                                                                                                                                                                                                                                                                                                                        | needs a Nix package (from nixpkgs, or an overlay-provided tool)                                     |
| Use when                 | the tool's version should travel with the module graph, and its dependency footprint is small to moderate                                                                                                                                                                                                                                                                                                 | the tool is heavy or codegen-only, doesn't need version-locking to `go.mod`, and you want it cached |

Reach for `nativeBuildInputs` for heavy codegen tools like `sqlc` that don't need to be pinned to your module's dependency graph. Reach for the `tool` directive when you want the tool's version to travel with `go.mod` and its dependency footprint is modest.

## Cross-compilation caveat

Invoking tools by their binary name (as above) works correctly in all scenarios including cross-compilation, because the host binary is not affected by `GOOS`/`GOARCH` overrides on the main build.

If you use `go tool <name>` directly — either in `preBuild` or via `//go:generate go tool <name>` directives — the Go toolchain compiles the tool from vendor using the current `GOOS`/`GOARCH` rather than using the pre-built host binary. Under cross-compilation this produces a target-platform binary that cannot execute on the build host. Prefer invoking tools directly by name to avoid this entirely.
