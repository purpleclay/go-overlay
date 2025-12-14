{buildGoApplication}: let
  version = "dev";
in
  buildGoApplication {
    pname = "go-scrape";
    inherit version;
    pwd = ./.;
    src = ./.;
    modules = ./gomod2nix.toml;
    subPackages = ["."];
    CGO_ENABLED = 0;
    doCheck = false;
  }
