# Usage: nix repl -f ./repl.nix
let
  flake = builtins.getFlake (toString ./.);
in
  import <nixpkgs> {
    overlays = [flake.overlays.default];
  }
