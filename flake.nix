{
  description = "A complete Go development environment for Nix. Toolchains, tools, and builders — pure, reproducible, and auto-updated";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs?ref=nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";

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
    git-hooks,
  }: let
    overlay = final: prev: {
      go-bin = import ./lib {
        lib = final.lib;
        pkgs = final;
      };

      # Builder for Go applications using govendor.toml
      inherit (final.callPackage ./builder {}) buildGoApplication buildGoWorkspace mkVendorEnv;
    };
  in
    {
      overlays.default = overlay;
      overlays.go-overlay = overlay;

      templates = {
        default = {
          path = ./templates/default;
          description = "A basic Go application using go-overlay";
        };
        workspace = {
          path = ./templates/workspace;
          description = "A Go workspace (go.work) application using go-overlay";
        };
      };

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
          overlays = [overlay];
        };

        devBuildInputs = with pkgs; [
          alejandra
          ((go-bin.fromGoMod ./go.mod).withTools [
            "gofumpt"
            "golangci-lint"
          ])
          nil
          self.packages.${system}.govendor
          typos
        ];

        pre-commit-check = git-hooks.lib.${system}.run {
          src = ./.;
          package = pkgs.prek;
          hooks = {
            alejandra = {
              enable = true;
              settings = {
                check = true;
              };
            };

            govendor = {
              enable = true;
              name = "govendor";
              description = "Check if govendor.toml has drifted from go.mod or go.work";
              entry = "${self.packages.${system}.govendor}/bin/govendor --check";
              files = "(^|/)go\\.(mod|work)$";
              excludes = [
                "examples/"
                "testdata/"
                "test/"
                "templates/"
              ];
              pass_filenames = true;
            };

            typos = {
              enable = true;
              entry = "${pkgs.typos}/bin/typos --force-exclude";
            };
          };
        };

        # Generate versioned package names (e.g., "1.25.5" -> "go_1_25_5", "1.25rc3" -> "go_1_25rc3")
        versionToPackageName = version: "go_" + builtins.replaceStrings ["."] ["_"] version;

        versionedPackages =
          pkgs.lib.mapAttrs' (
            version: drv: pkgs.lib.nameValuePair (versionToPackageName version) drv
          )
          pkgs.go-bin.versions;

        libTests = import ./test {inherit pkgs;};
        examples = import ./examples {
          inherit pkgs;
          go = pkgs.go-bin.fromGoMod ./go.mod;
        };
      in
        with pkgs; {
          checks =
            libTests
            // (pkgs.lib.filterAttrs (name: _: pkgs.lib.hasPrefix "example-" name) examples);

          devShells.default = mkShell {
            inherit (pre-commit-check) shellHook;
            buildInputs = devBuildInputs ++ pre-commit-check.enabledPackages;
          };

          packages =
            versionedPackages
            // examples
            // {
              default = pkgs.go-bin.latest;
              go = pkgs.go-bin.latest;
              goscrape = import ./goscrape.nix {
                inherit (pkgs) lib buildGoApplication;
                go = pkgs.go-bin.fromGoModStrict ./go.mod;
                commit = self.rev or "unknown";
              };
              govendor = import ./govendor.nix {
                inherit (pkgs) lib buildGoApplication;
                go = pkgs.go-bin.fromGoModStrict ./go.mod;
                commit = self.rev or "unknown";
              };
            };

          apps.default = {
            type = "app";
            program = "${self.packages.${system}.default}/bin/go";
            meta = {
              description = "The Go programming language";
              homepage = "https://go.dev/";
              license = lib.licenses.bsd3;
            };
          };

          apps.goscrape = {
            type = "app";
            program = "${self.packages.${system}.goscrape}/bin/goscrape";
            meta = {
              description = "A tool for scraping Go toolchains from https://go.dev/dl/";
              homepage = "https://github.com/purpleclay/go-overlay";
              license = licenses.mit;
              maintainers = with lib.maintainers; [purpleclay];
            };
          };

          apps.govendor = {
            type = "app";
            program = "${self.packages.${system}.govendor}/bin/govendor";
            meta = {
              description = "A tool for vendoring Go dependencies for a Go project";
              homepage = "https://github.com/purpleclay/go-overlay";
              license = licenses.mit;
              maintainers = with lib.maintainers; [purpleclay];
            };
          };
        }
    );
}
