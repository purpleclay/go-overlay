{
  pkgs,
  go,
}:
(pkgs.buildGoModule.override {inherit go;}) {
  pname = "go-vendor";
  version = "dev";
  src = ./.;
  subPackages = ["cmd/govendor"];
  env.CGO_ENABLED = 0;
  doCheck = false;
  vendorHash = "sha256-ig8Hs5uZ4CEplU/6YNDNNfBKy9FKa/Zcvt5dzpiVwhM=";
}
