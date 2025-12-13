{buildGoApplication}: let
  version = "dev";
in
  buildGoApplication {
    pname = "go-scrape";
    inherit version;
    pwd = ./scrape;
    src = ./scrape;
    modules = ./scrape/gomod2nix.toml;
    CGO_ENABLED = 0;
    doCheck = false;
  }
