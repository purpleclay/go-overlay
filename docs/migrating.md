# Migration Guides

## From gomod2nix

### Before

```nix
{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    gomod2nix.url = "github:nix-community/gomod2nix";
  };

  outputs = { nixpkgs, gomod2nix, ... }:
    let
      pkgs = import nixpkgs {
        system = "x86_64-linux";
        overlays = [ gomod2nix.overlays.default ];
      };
    in {
      packages.default = pkgs.buildGoApplication {
        pname = "myapp";
        version = "1.0.0";
        src = ./.;
        modules = ./gomod2nix.toml;
      };
    };
}
```

```bash
gomod2nix generate
```

### After

```nix
{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    go-overlay.url = "github:purpleclay/go-overlay";
  };

  outputs = { nixpkgs, go-overlay, ... }:
    let
      system = "x86_64-linux";
      pkgs = import nixpkgs {
        inherit system;
        overlays = [ go-overlay.overlays.default ];
      };
    in {
      packages.default = pkgs.buildGoApplication {
        pname = "myapp";
        version = "1.0.0";
        src = ./.;
        go = pkgs.go-bin.fromGoMod ./go.mod;
        modules = ./govendor.toml;
      };
    };
}
```

```bash
govendor
```

### Steps

1. Replace `gomod2nix` with `go-overlay` in flake inputs
2. Update the overlay reference
3. Add `go` parameter to `buildGoApplication`
4. Run `govendor` to generate the new manifest
5. Delete `gomod2nix.toml` and commit `govendor.toml`

## From buildGoModule

### Before

```nix
{
  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { nixpkgs, ... }:
    let
      pkgs = nixpkgs.legacyPackages.x86_64-linux;
    in {
      packages.default = pkgs.buildGoModule {
        pname = "myapp";
        version = "1.0.0";
        src = ./.;
        vendorHash = "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=";
      };
    };
}
```

### After

```nix
{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    go-overlay.url = "github:purpleclay/go-overlay";
  };

  outputs = { nixpkgs, go-overlay, ... }:
    let
      system = "x86_64-linux";
      pkgs = import nixpkgs {
        inherit system;
        overlays = [ go-overlay.overlays.default ];
      };
    in {
      packages.default = pkgs.buildGoApplication {
        pname = "myapp";
        version = "1.0.0";
        src = ./.;
        go = pkgs.go-bin.fromGoMod ./go.mod;
        modules = ./govendor.toml;
      };
    };
}
```

```bash
govendor
```

### Steps

1. Add `go-overlay` to flake inputs
2. Add the overlay to `pkgs`
3. Replace `buildGoModule` with `buildGoApplication`
4. Remove `vendorHash`
5. Add `go` and `modules` parameters
6. Run `govendor` to generate the manifest
7. Commit `govendor.toml`
