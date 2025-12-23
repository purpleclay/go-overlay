{
  buildGoApplication,
  go,
}:
buildGoApplication {
  pname = "govendor";
  version = "dev";
  src = ./.;
  modules = ./govendor.toml;
  inherit go;
  subPackages = ["cmd/govendor"];
  CGO_ENABLED = 0;
}
