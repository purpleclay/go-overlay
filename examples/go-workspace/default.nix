{
  pkgs,
  go,
}:
pkgs.buildGoWorkspace {
  inherit go;

  pname = "server";
  version = "0.1.0";
  src = ./.;

  # The workspace contains two modules (mood, server). subPackages selects
  # which one to build.
  subPackages = ["server"];
}
