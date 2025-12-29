{
  pkgs,
  go,
}:
pkgs.buildGoWorkspace {
  pname = "workspace-in-tree-vendor";
  version = "0.1.0";
  src = ./.;
  subPackages = ["api"];
  inherit go;
  # No modules parameter - uses in-tree vendor/ directory
}
