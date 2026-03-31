{
  pkgs,
  go,
}:
# No modules parameter — buildGoApplication detects the committed vendor/
# directory automatically and uses it in place of a govendor.toml manifest.
pkgs.buildGoApplication {
  inherit go;

  pname = "vendored";
  version = "0.1.0";
  src = ./.;
}
