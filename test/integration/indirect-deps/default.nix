{
  pkgs,
  go,
}:
pkgs.buildGoApplication {
  pname = "indirect-deps";
  version = "0.1.0";
  src = ./.;
  modules = ./govendor.toml;
  inherit go;
}
