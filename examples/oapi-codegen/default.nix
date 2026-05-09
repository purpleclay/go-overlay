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
  doCheck = false;

  # oapi-codegen is declared as a tool directive in go.mod. govendor compiles
  # it for the host platform and injects the binary into nativeBuildInputs,
  # making it available in $PATH here without any extra configuration.
  preBuild = ''
    oapi-codegen --config=api/oapi-codegen.yaml api/catto.yaml
  '';
}
