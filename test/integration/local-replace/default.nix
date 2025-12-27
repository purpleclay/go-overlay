{
  pkgs,
  go,
}:
pkgs.buildGoApplication {
  pname = "local-replace";
  version = "0.1.0";
  src = ./.;
  modules = ./govendor.toml;
  inherit go;
}
