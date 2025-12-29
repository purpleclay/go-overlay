{
  description = "Nix overlay for Go development. Pure, reproducible, auto-updated";

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
      inherit (final.callPackage ./builder {}) buildGoApplication mkVendorEnv;
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
          overlays = [overlay];
        };

        devBuildInputs = with pkgs; [
          alejandra
          go-bin.versions."1.25.4"
          gofumpt
          golangci-lint
          nil
          self.packages.${system}.govendor
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
              excludes = ["testdata/" "test/"];
              pass_filenames = true;
            };

            typos = {
              enable = true;
              entry = "${pkgs.typos}/bin/typos";
            };
          };
        };

        # Generate versioned package names (e.g., "1.25.5" -> "go_1_25_5", "1.25rc3" -> "go_1_25rc3")
        versionToPackageName = version:
          "go_" + builtins.replaceStrings ["."] ["_"] version;

        versionedPackages =
          pkgs.lib.mapAttrs'
          (version: drv: pkgs.lib.nameValuePair (versionToPackageName version) drv)
          pkgs.go-bin.versions;

        libTests = import ./test {inherit pkgs;};
      in
        with pkgs; {
          checks =
            {
              inherit pre-commit-check;
            }
            // libTests;

          devShells.default = mkShell {
            inherit (pre-commit-check) shellHook;
            buildInputs = devBuildInputs ++ pre-commit-check.enabledPackages;
          };

          packages =
            versionedPackages
            // {
              default = pkgs.go-bin.latest;
              go = pkgs.go-bin.latest;
              goscrape = import ./goscrape.nix {
                inherit (pkgs) buildGoApplication;
                go = pkgs.go-bin.fromGoModStrict ./go.mod;
              };
              govendor = import ./govendor.nix {
                inherit (pkgs) buildGoApplication;
                go = pkgs.go-bin.fromGoModStrict ./go.mod;
                commit = self.rev or "unknown";
              };
              integration-build-go-module = import ./test/integration/build-go-module {
                inherit pkgs;
                go = pkgs.go-bin.versions."1.22.3";
              };
              integration-local-replace = import ./test/integration/local-replace {
                inherit pkgs;
                go = pkgs.go-bin.versions."1.22.3";
              };
              # integration-in-tree-vendor is not exposed as a package because
              # it requires vendor/ to be generated first. It's built directly
              # in CI after running 'go mod vendor'. See .github/workflows/nix.yml
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
              homepage = "https://github.com/golang/go-overlay";
              license = licenses.mit;
              maintainers = with lib.maintainers; [purpleclay];
            };
          };

          apps.govendor = {
            type = "app";
            program = "${self.packages.${system}.govendor}/bin/govendor";
            meta = {
              description = "A tool for vendoring Go dependencies for a Go project";
              homepage = "https://github.com/golang/go-overlay";
              license = licenses.mit;
              maintainers = with lib.maintainers; [purpleclay];
            };
          };
        }
    );
}
