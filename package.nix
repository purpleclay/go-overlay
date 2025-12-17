{
  pkgs,
  go,
}:
(pkgs.buildGoModule.override {inherit go;}) {
  pname = "go-scrape";
  version = "dev";
  src = ./.;
  subPackages = ["."];
  env.CGO_ENABLED = 0;
  doCheck = false;
  vendorHash = "sha256-hgf8Oxb0gifbHKnlP/Yi258AGpdLe0HZm9lPieSCzlo=";
}
