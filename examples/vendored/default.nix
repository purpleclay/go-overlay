{
  pkgs,
  go,
}:
# No modules parameter — buildGoVendoredApplication detects the committed vendor/
# directory automatically and uses it in place of a govendor.toml manifest.
pkgs.buildGoVendoredApplication {
  inherit go;

  pname = "vendored";
  version = "0.1.0";
  src = ./.;
}
