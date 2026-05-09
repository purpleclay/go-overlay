{
  pkgs,
  go,
}:
pkgs.buildGoWorkspace {
  inherit go;

  pname = "server";
  version = "0.1.0";
  src = ./.;
  # For a workspace, govendor.toml includes a [workspace] section that records
  # the module graph and local replace directives alongside remote dependencies.
  modules = ./govendor.toml;
  # The workspace contains two modules (mood, server). subPackages selects
  # which one to build.
  subPackages = ["server"];
}
