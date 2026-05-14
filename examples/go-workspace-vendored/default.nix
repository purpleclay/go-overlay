{
  pkgs,
  go,
}:
# No govendor.toml — buildGoVendoredWorkspace uses the committed vendor/ directory directly.
pkgs.buildGoVendoredWorkspace {
  inherit go;

  pname = "server";
  version = "0.1.0";
  src = ../go-workspace;
  subPackages = ["server"];
}
