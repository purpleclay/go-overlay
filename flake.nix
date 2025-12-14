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
  }: let
    overlay = final: prev: {
      go-bin = import ./lib {
        lib = final.lib;
        pkgs = final;
      };
    };
  in
    {
      overlays.default = overlay;
      overlays.go-overlay = overlay;

      lib = {
        mkGoBin = pkgs:
          import ./lib {
            inherit (pkgs) lib;
            inherit pkgs;
          };
      };
    }
    // flake-utils.lib.eachDefaultSystem (
      system: let
        pkgs = import nixpkgs {
          inherit system;
          overlays = [
            gomod2nix.overlays.default
            overlay
          ];
        };

        devBuildInputs = with pkgs; [
          alejandra
          go-bin.versions."1.25.4"
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
            buildInputs = devBuildInputs ++ pre-commit-check.enabledPackages;
          };

          packages.default = pkgs.go-bin.latest;
          packages.go-scrape = pkgs.callPackage ./. {};

          apps.default = {
            type = "app";
            program = "${self.packages.${system}.default}/bin/go";
          };

          apps.go-scrape = {
            type = "app";
            program = "${self.packages.${system}.go-scrape}/bin/go-scrape";
          };
        }
    );
}
