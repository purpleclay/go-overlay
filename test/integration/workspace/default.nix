{
  pkgs,
  go,
}: {
  api = pkgs.buildGoWorkspace {
    pname = "workspace-api";
    version = "0.1.0";
    src = ./.;
    modules = ./govendor.toml;
    subPackages = ["api"];
    inherit go;
  };

  worker = pkgs.buildGoWorkspace {
    pname = "workspace-worker";
    version = "0.1.0";
    src = ./.;
    modules = ./govendor.toml;
    subPackages = ["worker"];
    inherit go;
  };
}
