{
  lib,
  buildGoApplication,
  go,
}: let
  pname = "govendor";
  version = "v1.6.2";
  buildDate = "2026-11-04T00:00:00Z";
in
  import ./go-cli.nix {
    inherit lib buildGoApplication go pname version buildDate;
    description = "Generate a vendor manifest for building Go applications with Nix";
  }
