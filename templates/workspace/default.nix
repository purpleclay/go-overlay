{
  pkgs,
  go,
}: let
  pname = "example";
in
  pkgs.buildGoWorkspace {
    inherit go pname;

    version = "0.1.0";
    src = ./.;
    subPackages = ["api"];

    meta.mainProgram = pname;
  }
