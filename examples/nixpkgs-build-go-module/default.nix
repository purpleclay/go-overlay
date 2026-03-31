{
  pkgs,
  go,
}:
# buildGoModule is nixpkgs' standard Go builder. Overriding its go attribute
# swaps in go-overlay's toolchain while keeping everything else from nixpkgs.
# Use this when you need nixpkgs ecosystem integration (e.g. NixOS modules,
# Home Manager packages) but still want go-overlay's Go version management.
(pkgs.buildGoModule.override {inherit go;}) {
  pname = "hello-world";
  version = "0.1.0";
  src = ../hello-world;
  vendorHash = null;
}
