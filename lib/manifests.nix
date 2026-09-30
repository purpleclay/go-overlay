# importManifest loads a single per-version manifest file. It exists so tests
# can prove which manifests an evaluation actually imports: version names and
# ordering come from filenames alone, so importing is deferred until a
# specific version's contents are needed.
{
  lib,
  importManifest ? import,
}: let
  inherit (import ./version.nix {inherit lib;}) parseVersion compareVersions;

  # Load all manifest files from the manifests directory
  manifestDir = ../manifests/go;
  manifestFiles = builtins.readDir manifestDir;

  # Filter to only per-version .nix files; index.nix is a generated summary,
  # not a version
  nixFiles =
    lib.filterAttrs
    (name: type: type == "regular" && lib.hasSuffix ".nix" name && name != "index.nix")
    manifestFiles;

  # Load each manifest, keyed by version string (filename without .nix)
  loadManifest = filename: importManifest (manifestDir + "/${filename}");

  # Create attribute set: { "1.21.6" = <manifest>; "1.25.5" = <manifest>; ... }
  manifests =
    lib.mapAttrs'
    (filename: _: let
      version = lib.removeSuffix ".nix" filename;
    in
      lib.nameValuePair version (loadManifest filename))
    nixFiles;

  # To identify the latest available version we must sorted all versions
  # in descending order and then take the first element
  sortedVersions = lib.sort (a: b: compareVersions a b > 0) (builtins.attrNames manifests);
  latest = builtins.head sortedVersions;

  # Filter out release candidates and betas to get the latest stable version
  stableVersions = builtins.filter (v: builtins.match ".*(rc|beta)[0-9]+" v == null) sortedVersions;
  latestStable = builtins.head stableVersions;

  # Get the latest patch version for a given minor version (e.g., "1.21" -> "1.21.13")
  latestPatch = minorVersion: let
    matching = builtins.filter (v: lib.hasPrefix "${minorVersion}." v) sortedVersions;
  in
    if matching == []
    then null
    else builtins.head matching;

  # Check if a version is deprecated according to Go's support policy.
  # Go supports the current and previous minor versions (e.g., 1.23.x and 1.22.x).
  # Applied uniformly by minor version regardless of pre-release stage — a
  # beta or rc is just as EOL as its stable sibling once its minor version
  # falls outside the supported window.
  isDeprecated = version: let
    parsed = parseVersion version;
    latestStableParsed = parseVersion latestStable;
    # Go supports N and N-1 minor versions
    minSupportedMinor = latestStableParsed.minor - 1;
  in
    parsed.major
    < latestStableParsed.major
    || (parsed.major == latestStableParsed.major && parsed.minor < minSupportedMinor);

  # Get all available versions for a given minor version (e.g., "1.21" -> ["1.21.13", "1.21.12", ...])
  versionsForMinor = minorVersion: let
    # Handle both "1.21" and "1.21.99" formats - extract "1.21"
    parts = lib.splitString "." minorVersion;
    prefix =
      if builtins.length parts >= 2
      then "${builtins.elemAt parts 0}.${builtins.elemAt parts 1}"
      else minorVersion;
    matching = builtins.filter (v: lib.hasPrefix "${prefix}." v || v == prefix) sortedVersions;
  in
    matching;
in {
  inherit manifests sortedVersions latest latestStable latestPatch isDeprecated versionsForMinor;
}
