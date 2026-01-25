{
  pkgs,
  go,
}:
pkgs.buildGoApplication {
  pname = "integration-local-replace-external";
  version = "0.1.0";
  src = ./examples;
  modules = ./examples/govendor.toml;
  inherit go;

  # Provide Nix path for local replace directive that points to parent directory
  # The govendor.toml has: local = "../"
  localReplaces = {
    "example.com/integration-local-replace-external" = ./.;
  };
}
