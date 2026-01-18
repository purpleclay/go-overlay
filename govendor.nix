{
  buildGoApplication,
  go,
  commit ? "unknown",
}: let
  version = "v0.8.1";
  buildDate = "2026-01-18T00:00:00Z";
in
  buildGoApplication {
    inherit version go;

    pname = "govendor";
    src = ./.;
    modules = ./govendor.toml;
    subPackages = ["cmd/govendor"];
    CGO_ENABLED = 0;
    ldflags = [
      "-s"
      "-w"
      "-X main.Version=${version}"
      "-X main.Commit=${commit}"
      "-X main.BuildDate=${buildDate}"
    ];
  }
