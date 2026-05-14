{
  pkgs,
  go,
}:
pkgs.buildGoApplication {
  inherit go;

  pname = "http-chi-server";
  version = "0.1.0";
  src = ./.;
}
