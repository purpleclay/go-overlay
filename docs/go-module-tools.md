# Go Module Tool Directives

Go 1.24 introduced `tool` directives in `go.mod` as a first-class way to pin code-generation and other build tools alongside your module dependencies:

```
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

## Cross-compilation caveat

Invoking tools by their binary name (as above) works correctly in all scenarios including cross-compilation, because the host binary is not affected by `GOOS`/`GOARCH` overrides on the main build.

If you use `go tool <name>` directly — either in `preBuild` or via `//go:generate go tool <name>` directives — the Go toolchain compiles the tool from vendor using the current `GOOS`/`GOARCH` rather than using the pre-built host binary. Under cross-compilation this produces a target-platform binary that cannot execute on the build host. Prefer invoking tools directly by name to avoid this entirely.
