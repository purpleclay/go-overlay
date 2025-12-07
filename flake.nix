{
  description = "Pure and reproducible nix overlay of binary distributed golang toolchains";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs?ref=nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";

    gomod2nix = {
      url = "github:nix-community/gomod2nix";
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
    gomod2nix,
    git-hooks,
  }:
    flake-utils.lib.eachDefaultSystem (
      system: let
        pkgs = import nixpkgs {
          inherit system;
          overlays = [gomod2nix.overlays.default];
        };

        buildInputs = with pkgs; [
          alejandra
          go
          gofumpt
          golangci-lint
          gomod2nix.packages.${system}.default
          nil
        ];

        pre-commit-check = git-hooks.lib.${system}.run {
          src = ./.;
          package = pkgs.prek;
          hooks = {
            typos = {
              enable = true;
              entry = "${pkgs.typos}/bin/typos";
            };
          };
        };
      in
        with pkgs; {
          checks = {
            inherit pre-commit-check;
          };

          devShells.default = mkShell {
            inherit (pre-commit-check) shellHook;
            buildInputs = buildInputs ++ pre-commit-check.enabledPackages;
          };

          packages.default = pkgs.callPackage ./. {};

          apps.default = {
            type = "app";
            program = "${self.packages.${system}.default}/bin/go-scrape";
          };
        }
    );
}
