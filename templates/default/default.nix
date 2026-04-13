{
  pkgs,
  go,
}:
pkgs.buildGoApplication {
  pname = "example";
  version = "0.1.0";
  src = ./.;
  inherit go;
  modules = ./govendor.toml;
}
