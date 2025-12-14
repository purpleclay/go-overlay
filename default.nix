# Overlay for traditional (non-flake) Nix usage
# This can be imported directly or added to nixpkgs overlays
final: prev: {
  go-bin = import ./lib {
    lib = final.lib;
    pkgs = final;
  };
}
