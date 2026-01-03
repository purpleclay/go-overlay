{
  pkgs,
  go,
}:
pkgs.buildGoWorkspace {
  pname = "workspace-api";
  version = "0.1.0";
  src = ./.;
  modules = ./govendor.toml;
  subPackages = ["api"];
  inherit go;
}
