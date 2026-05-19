# go-workspace

A daily mood check-in built with HTMX, demonstrating `buildGoWorkspace` for multi-module Go workspaces with a shared library and an HTTP server.

## Getting started

Run the example:

```shell
nix run .#example-go-workspace
```

Then open [http://localhost:8080](http://localhost:8080) in your browser.

## The Nix bit

```nix
pkgs.buildGoWorkspace {
  inherit go;

  pname = "server";
  version = "0.1.0";
  src = ./.;

  # The workspace contains two modules (mood, server). subPackages selects
  # which one to build.
  subPackages = ["server"];
}
```
