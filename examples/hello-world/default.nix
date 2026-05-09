{
  pkgs,
  go,
}:
pkgs.buildGoApplication {
  inherit go;

  pname = "hello-world";
  version = "0.1.0";
  src = ./.;
  doCheck = false;
}
