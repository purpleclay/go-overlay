{
  buildGoApplication,
  go,
  commit ? "unknown",
}: let
  version = "v0.2.0";
  buildDate = "2026-03-17T00:00:00Z";
in
  buildGoApplication {
    inherit version go;

    pname = "goscrape";
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
  }
