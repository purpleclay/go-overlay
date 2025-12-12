{lib}: let
  # Load all manifest files from the manifests directory
  manifestDir = ../manifests;
  manifestFiles = builtins.readDir manifestDir;

  # Filter to only .nix files and strip the extension to get version
  nixFiles =
    lib.filterAttrs
    (name: type: type == "regular" && lib.hasSuffix ".nix" name)
    manifestFiles;

  # Load each manifest, keyed by version string (filename without .nix)
  loadManifest = filename: import (manifestDir + "/${filename}");

  # Create attribute set: { "1.21.6" = <manifest>; "1.25.5" = <manifest>; ... }
  manifests =
    lib.mapAttrs'
    (filename: _: let
      version = lib.removeSuffix ".nix" filename;
    in
      lib.nameValuePair version (loadManifest filename))
    nixFiles;

  # Parse version string into components (major, minor, patch, rc) for comparison
  # Handles both stable versions (1.21.6) and release candidates (1.25rc1)
  parseVersion = v: let
    parts = lib.splitString "." v;
    major = lib.toInt (builtins.elemAt parts 0);

    # The minor component may contain an "rc" suffix (e.g., "25rc1")
    minorPart = builtins.elemAt parts 1;
    hasRc = builtins.match "([0-9]+)rc([0-9]+)" minorPart;
  in
    if hasRc != null
    then {
      inherit major;
      minor = lib.toInt (builtins.elemAt hasRc 0);
      patch = 0;
      # RC versions sort before stable, so use negative values
      # -1000 + rcNum ensures rc1 < rc2 < rc3 < stable (patch 0)
      rc = -1000 + lib.toInt (builtins.elemAt hasRc 1);
    }
    else {
      inherit major;
      minor = lib.toInt minorPart;
      patch =
        if builtins.length parts > 2
        then lib.toInt (builtins.elemAt parts 2)
        else 0;
      rc = 0;
    };

  # Compare two version strings
  compareVersions = a: b: let
    va = parseVersion a;
    vb = parseVersion b;
  in
    if va.major != vb.major
    then va.major - vb.major
    else if va.minor != vb.minor
    then va.minor - vb.minor
    else if va.patch != vb.patch
    then va.patch - vb.patch
    else va.rc - vb.rc;

  # To identify the latest available version we must sorted all versions
  # in descending order and then take the first element
  sortedVersions = lib.sort (a: b: compareVersions a b > 0) (builtins.attrNames manifests);
  latest = builtins.head sortedVersions;

  # Get the latest patch version for a given minor version (e.g., "1.21" -> "1.21.13")
  latestPatch = minorVersion: let
    matching = builtins.filter (v: lib.hasPrefix "${minorVersion}." v) sortedVersions;
  in
    if matching == []
    then null
    else builtins.head matching;
in {
  inherit manifests latest latestPatch;
}
