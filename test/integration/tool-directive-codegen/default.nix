{
  pkgs,
  go,
}:
pkgs.buildGoApplication {
  pname = "tool-directive-codegen";
  version = "0.1.0";
  src = ./.;
  modules = ./govendor.toml;
  inherit go;
  preBuild = ''
    go generate ./...
  '';
}
