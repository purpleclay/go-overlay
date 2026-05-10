{
  lib,
  buildGoApplication,
  go,
  commit ? "unknown",
}: let
  pname = "govendor";
  version = "v0.13.0";
  buildDate = "2026-05-07T00:00:00Z";
in
  buildGoApplication {
    inherit pname version go;
    src = ./.;
    modules = ./govendor.toml;
    subPackages = ["cmd/govendor"];
    CGO_ENABLED = 0;

    # Integration tests in internal/vendor and internal/resolve shell out to
    # `go mod download`, which needs network access. The Nix sandbox enforces
    # GOPROXY=off, so these tests cannot run here. CI runs the full suite via
    # `go test` outside the sandbox.
    doCheck = false;
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
