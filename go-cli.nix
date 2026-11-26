# Builds one of the repository's own Go CLIs (govendor, goscrape).
#
# src holds only what the Go build reads. Taking the whole repository would
# pull in manifests/, which automated pull requests change several times a
# week, and every such merge would change the derivation and miss the binary
# cache for `nix run github:purpleclay/go-overlay#<cli>`.
{
  lib,
  buildGoApplication,
  go,
  pname,
  version,
  buildDate,
  description,
}:
buildGoApplication {
  inherit pname version go;
  src = lib.fileset.toSource {
    root = ./.;
    fileset = lib.fileset.unions [
      ./go.mod
      ./go.sum
      ./govendor.toml
      ./internal
      ./cmd/${pname}
    ];
  };
  subPackages = ["cmd/${pname}"];
  CGO_ENABLED = 0;

  # Some integration tests shell out to `go mod download`, which needs network
  # access. The Nix sandbox enforces GOPROXY=off, so these tests cannot run
  # here. CI runs the full suite via `go test` outside the sandbox.
  doCheck = false;
  ldflags = [
    "-s"
    "-w"
    "-X main.Version=${version}"
    "-X main.BuildDate=${buildDate}"
  ];

  meta = with lib; {
    homepage = "https://github.com/purpleclay/go-overlay";
    inherit description;
    mainProgram = pname;
    license = licenses.mit;
    maintainers = with maintainers; [purpleclay];
  };
}
