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

  # Parse version string into integer components (major, minor, patch) for comparison
  parseVersion = v: let
    parts = lib.splitString "." v;
  in {
    major = lib.toInt (builtins.elemAt parts 0);
    minor = lib.toInt (builtins.elemAt parts 1);
    patch =
      if builtins.length parts > 2
      then lib.toInt (builtins.elemAt parts 2)
      else 0;
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
    else va.patch - vb.patch;

  # To identify the latest available version we must sorted all versions
  # in descending order and then take the first element
  sortedVersions = lib.sort (a: b: compareVersions a b > 0) (builtins.attrNames manifests);
  latest = builtins.head sortedVersions;
in {
  inherit manifests latest;
}
