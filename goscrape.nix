{
  lib,
  buildGoApplication,
  go,
  commit ? "unknown",
}: let
  pname = "goscrape";
  version = "v0.2.0";
  buildDate = "2026-03-17T00:00:00Z";
in
  buildGoApplication {
    inherit pname version go;
    src = ./.;
    modules = ./govendor.toml;
    subPackages = ["cmd/goscrape"];
    CGO_ENABLED = 0;
    ldflags = [
      "-s"
      "-w"
      "-X main.Version=${version}"
      "-X main.Commit=${commit}"
      "-X main.BuildDate=${buildDate}"
    ];

    meta = with lib; {
      homepage = "https://github.com/purpleclay/go-overlay";
      description = "Tools for scraping Go releases and generating Nix manifests";
      mainProgram = pname;
      license = licenses.mit;
      maintainers = with maintainers; [purpleclay];
    };
  }
