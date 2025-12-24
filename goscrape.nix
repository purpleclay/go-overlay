{
  buildGoApplication,
  go,
}:
buildGoApplication {
  pname = "goscrape";
  version = "dev";
  src = ./.;
  modules = ./govendor.toml;
  inherit go;
  subPackages = ["cmd/goscrape"];
  CGO_ENABLED = 0;
}
