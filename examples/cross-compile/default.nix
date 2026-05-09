{
  pkgs,
  go,
  GOOS ? go.GOOS,
  GOARCH ? go.GOARCH,
}:
pkgs.buildGoApplication {
  inherit go GOOS GOARCH;

  pname = "cross-compile";
  version = "0.1.0";
  src = ./.;
  modules = ./govendor.toml;
  doCheck = false;

  # CGO_ENABLED = "0" is required for cross-compilation. Pure-Go binaries need
  # no C cross-compiler, no sysroot — just the Go toolchain.
  CGO_ENABLED = "0";
}
