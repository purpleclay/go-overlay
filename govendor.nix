{
  buildGoApplication,
  go,
  commit ? "unknown",
}: let
  version = "v0.2.0";
  buildDate = "2025-12-27T00:00:00Z";
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
