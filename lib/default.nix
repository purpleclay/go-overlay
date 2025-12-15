# The main entry point for go-overlay, providing the go-bin attribute set with version selection
{
  lib,
  pkgs,
}: let
  manifestsLib = import ./manifests.nix {inherit lib;};
  mkGoToolchain = import ./mk-go-toolchain.nix {
    inherit lib;
    inherit (pkgs) stdenv fetchurl;
  };

  allVersions =
    lib.mapAttrs
    (version: manifest: mkGoToolchain manifest)
    manifestsLib.manifests;

  # Parse a go.mod file and extract both go and toolchain versions.
  # Toolchains were introduced in Go 1.21, https://go.dev/doc/toolchain
  parseGoMod = path: let
    content = builtins.readFile path;

    # Match "go X.Y" or "go X.Y.Z" on its own line
    goMatch = builtins.match ".*\ngo ([0-9]+\\.[0-9]+(\\.[0-9]+)?)\n.*" content;

    # Match "toolchain goX.Y.Z" on its own line
    toolchainMatch = builtins.match ".*\ntoolchain go([0-9]+\\.[0-9]+\\.[0-9]+)\n.*" content;
  in {
    go =
      if goMatch != null
      then builtins.head goMatch
      else null;
    toolchain =
      if toolchainMatch != null
      then builtins.head toolchainMatch
      else null;
  };

  # Resolve version string to a derivation, with optional fallback to latest patch
  resolveVersion = version: fallbackToLatestPatch: let
    exact = allVersions.${version} or null;

    # Try to find latest patch if version is minor only (e.g., "1.21")
    latestPatchVersion = manifestsLib.latestPatch version;
    latestPatch =
      if latestPatchVersion != null
      then allVersions.${latestPatchVersion}
      else null;
  in
    if exact != null
    then exact
    else if fallbackToLatestPatch && latestPatch != null
    then latestPatch
    else throw "go-overlay: Go version '${version}' is not available";

  # Use toolchain if present, otherwise resolve go version (with latest patch fallback)
  fromGoMod = path: let
    parsed = parseGoMod path;
  in
    if parsed.toolchain != null
    then resolveVersion parsed.toolchain false
    else if parsed.go != null
    then resolveVersion parsed.go true
    else throw "go-overlay: Could not parse Go version from ${toString path}";

  # Use exactly what's specified (toolchain takes precedence, no latest patch fallback)
  fromGoModStrict = path: let
    parsed = parseGoMod path;
  in
    if parsed.toolchain != null
    then resolveVersion parsed.toolchain false
    else if parsed.go != null
    then resolveVersion parsed.go false
    else throw "go-overlay: Could not parse Go version from ${toString path}";
in {
  latest = allVersions.${manifestsLib.latest};
  latestStable = allVersions.${manifestsLib.latestStable};
  versions = allVersions;
  inherit fromGoMod fromGoModStrict;
}
