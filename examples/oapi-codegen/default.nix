{
  pkgs,
  go,
}:
pkgs.buildGoApplication {
  inherit go;

  pname = "oapi-codegen";
  version = "0.1.0";
  src = ./.;
  modules = ./govendor.toml;

  # oapi-codegen is declared as a tool directive in go.mod rather than a
  # nativeBuildInput. govendor includes tool dependencies in govendor.toml
  # so the Go toolchain can find and run it from the vendor directory.
  preBuild = ''
    go generate ./...
  '';
}
