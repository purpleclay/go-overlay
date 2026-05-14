{
  pkgs,
  go,
  tags ? [],
}:
pkgs.buildGoApplication {
  inherit go tags;

  pname = "build-tags";
  version = "0.1.0";
  src = ./.;
}
