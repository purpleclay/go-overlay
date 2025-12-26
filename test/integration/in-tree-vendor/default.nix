# Integration test for in-tree vendor support
# Tests that buildGoApplication can build a project with a committed vendor/ directory
{
  pkgs,
  go,
}:
pkgs.buildGoApplication {
  pname = "integration-vendor-test";
  version = "0.1.0";
  src = ./.;
  inherit go;
  # No modules parameter - uses in-tree vendor/
}
