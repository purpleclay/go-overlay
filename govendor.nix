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
  vendorHash = "sha256-bLfVDTHC47JhIJgAVmpFSWSfyIJCltpy8ntfDCs19/w=";
}
