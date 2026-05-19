{
  pkgs,
  app,
}:
pkgs.dockerTools.buildLayeredImage {
  name = "ping-server";
  tag = "latest";
  config = {
    Cmd = [(pkgs.lib.getExe app)];
    ExposedPorts = {
      "8080/tcp" = {};
    };
  };
}
