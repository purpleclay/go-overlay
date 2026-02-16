# Loads tool manifests from manifests/<tool>/ directories.
# Each subdirectory under manifests/ (except "go/") is treated as a tool.
{lib}: let
  manifestBase = ../manifests;
  allEntries = builtins.readDir manifestBase;

  # Tool directories are any directory under manifests/ that isn't "go"
  toolDirs =
    lib.filterAttrs
    (name: type: type == "directory" && name != "go")
    allEntries;

  # Parse semver "major.minor.patch" into comparable components
  parseToolVersion = v: let
    parts = lib.splitString "." v;
  in {
    major = lib.toInt (builtins.elemAt parts 0);
    minor = lib.toInt (builtins.elemAt parts 1);
    patch =
      if builtins.length parts > 2
      then lib.toInt (builtins.elemAt parts 2)
      else 0;
  };

  compareToolVersions = a: b: let
    va = parseToolVersion a;
    vb = parseToolVersion b;
  in
    if va.major != vb.major
    then va.major - vb.major
    else if va.minor != vb.minor
    then va.minor - vb.minor
    else va.patch - vb.patch;

  # Load all manifests for a single tool directory
  loadTool = toolName: let
    toolDir = manifestBase + "/${toolName}";
    files = builtins.readDir toolDir;
    nixFiles =
      lib.filterAttrs
      (name: type: type == "regular" && lib.hasSuffix ".nix" name)
      files;

    manifests =
      lib.mapAttrs'
      (filename: _: let
        version = lib.removeSuffix ".nix" filename;
      in
        lib.nameValuePair version (import (toolDir + "/${filename}")))
      nixFiles;

    sortedVersions = lib.sort (a: b: compareToolVersions a b > 0) (builtins.attrNames manifests);
  in {
    inherit manifests sortedVersions;
    latest = builtins.head sortedVersions;
  };

  tools = lib.mapAttrs (name: _: loadTool name) toolDirs;
in {
  inherit tools;
}
