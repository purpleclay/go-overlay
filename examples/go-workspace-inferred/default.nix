{
  pkgs,
  go,
}:
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
