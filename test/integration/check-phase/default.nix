{
  pkgs,
  go,
}:
pkgs.buildGoApplication {
  pname = "check-phase";
  version = "0.1.0";
  src = ./.;
  modules = ./govendor.toml;
  inherit go;
  doCheck = true;
}
