{
  buildGoApplication,
  go,
  commit ? "unknown",
}: let
  version = "v0.1.1";
  buildDate = "2026-02-21T00:00:00Z";
in
  buildGoApplication {
    inherit version go;

    pname = "goscrapeproxy";
    src = ./.;
    modules = ./govendor.toml;
    subPackages = ["cmd/goscrapeproxy"];
    CGO_ENABLED = 0;
    ldflags = [
      "-s"
      "-w"
      "-X main.Version=${version}"
      "-X main.Commit=${commit}"
      "-X main.BuildDate=${buildDate}"
    ];
  }
