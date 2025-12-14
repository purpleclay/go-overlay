{
  pkgs,
  go,
}:
(pkgs.buildGoModule.override {inherit go;}) {
  pname = "integration-test";
  version = "0.1.0";
  src = ./.;
  vendorHash = null;
}
