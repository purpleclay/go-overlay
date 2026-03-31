{
  pkgs,
  go,
}:
# No modules parameter — buildGoWorkspace detects the committed vendor/ directory automatically.
pkgs.buildGoWorkspace {
  inherit go;

  pname = "server";
  version = "0.1.0";
  src = ../go-workspace;
  subPackages = ["server"];
}
