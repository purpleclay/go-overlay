{
  pkgs,
  go,
}:
(pkgs.buildGoModule.override {inherit go;}) {
  pname = "build-go-module";
  version = "0.1.0";
  src = ./.;
  vendorHash = null;
}
