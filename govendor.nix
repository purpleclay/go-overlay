{
  pkgs,
  go,
}:
(pkgs.buildGoModule.override {inherit go;}) {
  pname = "govendor";
  version = "dev";
  src = ./.;
  subPackages = ["cmd/govendor"];
  env.CGO_ENABLED = 0;
  doCheck = false;
  vendorHash = "sha256-lS1x30VeP4A+uYgdi+BYSck17reGLdWXV1llVln14Is=";
}
