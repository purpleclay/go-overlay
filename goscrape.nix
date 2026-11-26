{
  lib,
  buildGoApplication,
  go,
}: let
  pname = "goscrape";
  version = "v0.6.1";
  buildDate = "2026-11-04T00:00:00Z";
in
  import ./go-cli.nix {
    inherit lib buildGoApplication go pname version buildDate;
    description = "Tools for scraping Go releases and generating Nix manifests";
  }
