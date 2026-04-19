{
  description = "A Go workspace application";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";

    go-overlay = {
      url = "github:purpleclay/go-overlay";
      inputs = {
        nixpkgs.follows = "nixpkgs";
        flake-utils.follows = "flake-utils";
      };
    };

    git-hooks = {
      url = "github:cachix/git-hooks.nix";
      inputs = {
        nixpkgs.follows = "nixpkgs";
      };
    };
  };

  outputs = {
    self,
    nixpkgs,
    flake-utils,
    go-overlay,
    git-hooks,
    ...
  }:
    flake-utils.lib.eachDefaultSystem (
      system: let
        pkgs = import nixpkgs {
          inherit system;
          overlays = [go-overlay.overlays.default];
        };
        go = pkgs.go-bin.fromGoMod ./api/go.mod;

        pre-commit-check = git-hooks.lib.${system}.run {
          src = ./.;
          hooks.govendor = {
            enable = true;
            name = "govendor";
            entry = "${go-overlay.packages.${system}.govendor}/bin/govendor --check";
            files = "(^|/)go\\.(mod|work)$";
            pass_filenames = true;
          };
        };
      in
        with pkgs; {
          packages.default = callPackage ./default.nix {inherit go;};

          devShells.default = mkShell {
            inherit (pre-commit-check) shellHook;
            buildInputs =
              [
                go.withDefaultTools
                go-overlay.packages.${system}.govendor
              ]
              ++ pre-commit-check.enabledPackages;
          };
        }
    );
}
