# go-workspace-inferred

Builds the same mood check-in app as `go-workspace` but without a `go.work` file in the source tree. go-overlay infers the workspace structure from `govendor.toml` and generates `go.work` inside the build sandbox.

## Getting started

Run the example:

```shell
nix run .#example-go-workspace-inferred
```

Then open [http://localhost:8080](http://localhost:8080) in your browser.

## The Nix bit

```nix
# There is no go.work in this source tree — buildGoWorkspace infers the workspace
# structure from the govendor.toml manifest and generates go.work at build time.
pkgs.buildGoWorkspace {
  inherit go;

  pname = "server";
  version = "0.1.0";
  # Filter out go.work so go-overlay generates it from the manifest instead.
  src = pkgs.lib.cleanSourceWith {
    src = ../go-workspace;
    filter = path: _type: builtins.baseNameOf path != "go.work";
  };
  modules = ../go-workspace/govendor.toml;
  subPackages = ["server"];
}
```

`lib.cleanSourceWith` filters the source tree before it enters the Nix sandbox. Removing `go.work` causes `buildGoWorkspace` to generate one from the `[workspace]` section of `govendor.toml`. Use this pattern for projects that deliberately don't commit `go.work` — the manifest carries the workspace metadata so Nix builds remain reproducible.

During the build, `buildGoWorkspace` logs which path was taken:

```shell
go-overlay: generating go.work from govendor.toml
```
