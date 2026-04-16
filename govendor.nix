{
  lib,
  buildGoApplication,
  go,
  commit ? "unknown",
}: let
  pname = "govendor";
  version = "v0.9.4";
  buildDate = "2026-03-29T00:00:00Z";
in
  buildGoApplication {
    inherit pname version go;
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

    meta = with lib; {
      homepage = "https://github.com/purpleclay/go-overlay";
      description = "Generate a vendor manifest for building Go applications with Nix";
      mainProgram = pname;
      license = licenses.mit;
      maintainers = with maintainers; [purpleclay];
    };
  }
