{
  pkgs,
  go,
}:
(pkgs.buildGoModule.override {inherit go;}) {
  pname = "goscrape";
  version = "dev";
  src = ./.;
  subPackages = ["cmd/goscrape"];
  env.CGO_ENABLED = 0;
  doCheck = false;
  vendorHash = "sha256-lS1x30VeP4A+uYgdi+BYSck17reGLdWXV1llVln14Is=";
}
