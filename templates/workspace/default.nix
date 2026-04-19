{
  pkgs,
  go,
}:
pkgs.buildGoWorkspace {
  pname = "api";
  version = "0.1.0";
  src = ./.;
  inherit go;
  modules = ./govendor.toml;
  subPackages = ["api"];
}
