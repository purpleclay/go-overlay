{
  description = "Pure and reproducible nix overlay of binary distributed golang toolchains";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs?ref=nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = {
    self,
    nixpkgs,
    flake-utils,
  }:
    flake-utils.lib.eachDefaultSystem (
      system: let
        pkgs = nixpkgs.legacyPackages.${system};
      in
        with pkgs; {
          devShells.default = mkShell {
            buildInputs = [
              alejandra
              go
              nil
            ];
          };
        }
    );
}
