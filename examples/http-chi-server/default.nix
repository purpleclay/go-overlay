{
  pkgs,
  go,
}:
pkgs.buildGoApplication {
  inherit go;

  pname = "http-chi-server";
  version = "0.1.0";
  src = ./.;
  # The modules parameter points buildGoApplication at the govendor.toml manifest,
  # which pins each external dependency with a hash for integrity verification.
  modules = ./govendor.toml;
}
