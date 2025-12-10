# The main entry point for go-overlay, providing the go-bin attribute set with version selection
{
  lib,
  pkgs,
}: let
  manifestsLib = import ./manifests.nix {inherit lib;};
  mkGoToolchain = import ./mk-go-toolchain.nix {
    inherit lib;
    inherit (pkgs) stdenv fetchurl;
  };

  allVersions =
    lib.mapAttrs
    (version: manifest: mkGoToolchain manifest)
    manifestsLib.manifests;
in {
  latest = allVersions.${manifestsLib.latest};
  versions = allVersions;
}
