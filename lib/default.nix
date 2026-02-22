# The main entry point for go-overlay, providing the go-bin attribute set with version selection
{
  lib,
  pkgs,
}: let
  manifestsLib = import ./manifests.nix {inherit lib;};
  toolManifestsLib = import ./tool-manifests.nix {inherit lib;};

  # Get builder functions for tool building
  builder = pkgs.callPackage ../builder {};

  buildGoTool = import ./mk-go-tool.nix {
    inherit lib;
    inherit (pkgs) stdenv;
    inherit (builder) fetchGoModule mkVendorEnv;
  };

  mkToolSet = import ./mk-tool-set.nix {
    inherit lib buildGoTool;
    toolManifests = toolManifestsLib;
  };

  mkGoToolchain = import ./mk-go-toolchain.nix {
    inherit lib mkToolSet;
    inherit (pkgs) stdenv fetchurl symlinkJoin;
  };

  allVersions =
    lib.mapAttrs
    (version: manifest: mkGoToolchain manifest)
    manifestsLib.manifests;

  # Parse a go.mod file and extract both go and toolchain versions.
  # Toolchains were introduced in Go 1.21, https://go.dev/doc/toolchain
  #
  # Uses line-based parsing to reliably match the first occurrence of each
  # directive, avoiding issues with greedy regex matching across multiple lines.
  parseGoMod = path: let
    content = builtins.readFile path;
    lines = builtins.filter builtins.isString (builtins.split "\n" content);

    # Find first line matching "go X.Y", "go X.Y.Z", or "go X.YrcN" exactly
    goLines = builtins.filter (line: builtins.match "go [0-9]+\\.[0-9]+((\\.[0-9]+)|(rc[0-9]+))?" line != null) lines;
    firstGoLine =
      if goLines != []
      then builtins.head goLines
      else null;
    goMatch =
      if firstGoLine != null
      then builtins.match "go ([0-9]+\\.[0-9]+((\\.[0-9]+)|(rc[0-9]+))?)" firstGoLine
      else null;

    # Find first line matching "toolchain goX.Y.Z" or "toolchain goX.YrcN" exactly
    toolchainLines = builtins.filter (line: builtins.match "toolchain go[0-9]+\\.[0-9]+((\\.[0-9]+)|(rc[0-9]+))" line != null) lines;
    firstToolchainLine =
      if toolchainLines != []
      then builtins.head toolchainLines
      else null;
    toolchainMatch =
      if firstToolchainLine != null
      then builtins.match "toolchain go([0-9]+\\.[0-9]+((\\.[0-9]+)|(rc[0-9]+)))" firstToolchainLine
      else null;
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

    # Build helpful error message with available versions
    availableInSeries = manifestsLib.versionsForMinor version;
    availableMsg =
      if availableInSeries != []
      then "\n\n  Available versions: ${lib.concatStringsSep ", " availableInSeries}"
      else "";
    suggestions = ''

      Suggestions:
        - Use 'go-bin.fromGoMod ./go.mod' for automatic version selection
        - Run 'nix flake update go-overlay' to get newly released versions'';
  in
    if exact != null
    then exact
    else if fallbackToLatestPatch && latestPatch != null
    then latestPatch
    else throw "go-overlay: Go version '${version}' is not available${availableMsg}${suggestions}";

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

  # Check if an exact version is available
  hasVersion = version: builtins.hasAttr version allVersions;

  # Check if a version is deprecated (EOL) according to Go's support policy
  isDeprecated = manifestsLib.isDeprecated;
in {
  latest = allVersions.${manifestsLib.latest};
  latestStable = allVersions.${manifestsLib.latestStable};
  versions = allVersions;
  inherit fromGoMod fromGoModStrict hasVersion isDeprecated;
}
