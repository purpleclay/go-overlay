{
  description = "A basic Go application";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";

    go-overlay.url = "github:purpleclay/go-overlay";
    go-overlay.inputs.nixpkgs.follows = "nixpkgs";
    go-overlay.inputs.flake-utils.follows = "flake-utils";
  };

  outputs = {
    self,
    nixpkgs,
    flake-utils,
    go-overlay,
    ...
  }:
    flake-utils.lib.eachDefaultSystem (
      system: let
        pkgs = import nixpkgs {
          inherit system;
          overlays = [go-overlay.overlays.default];
        };
        go = pkgs.go-bin.fromGoMod ./go.mod;
      in {
        packages.default = pkgs.buildGoApplication {
          pname = "example";
          version = "0.1.0";
          src = ./.;
          inherit go;
          modules = ./govendor.toml;
        };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go.withDefaultTools
            go-overlay.packages.${system}.govendor
          ];
        };
      }
    );
}
