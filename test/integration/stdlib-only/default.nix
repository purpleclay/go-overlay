{
  pkgs,
  go,
}:
pkgs.buildGoApplication {
  pname = "stdlib-only";
  version = "0.1.0";
  src = ./.;
  inherit go;
}
