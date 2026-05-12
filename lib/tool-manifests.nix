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

  # Parse semver "major.minor.patch" into comparable components.
  # Handles pre-release suffixes like "0.22.0-pre.2" where the patch segment
  # contains a hyphen (e.g. "0-pre") followed by an optional numeric pre-release
  # counter as the next dot-separated part.
  parseToolVersion = v: let
    parts = lib.splitString "." v;
    patchStr =
      if builtins.length parts > 2
      then builtins.elemAt parts 2
      else "0";
    # Split "0-pre" → ["0" "pre"]; plain "3" → ["3"]
    patchParts = lib.splitString "-" patchStr;
    isPreRelease = builtins.length patchParts > 1;
    # The numeric counter after the last dot in a pre-release, e.g. the "2" in "0.22.0-pre.2".
    # Guard against non-numeric suffixes (e.g. -pre.rc1) to avoid a toInt evaluation crash.
    preNum =
      if isPreRelease && builtins.length parts > 3
      then let
        prePart = builtins.elemAt parts 3;
      in
        if builtins.match "[0-9]+" prePart != null
        then lib.toInt prePart
        else 0
      else 0;
  in {
    major = lib.toInt (builtins.elemAt parts 0);
    minor = lib.toInt (builtins.elemAt parts 1);
    patch = lib.toInt (builtins.head patchParts);
    inherit isPreRelease preNum;
  };

  compareToolVersions = a: b: let
    va = parseToolVersion a;
    vb = parseToolVersion b;
  in
    if va.major != vb.major
    then va.major - vb.major
    else if va.minor != vb.minor
    then va.minor - vb.minor
    else if va.patch != vb.patch
    then va.patch - vb.patch
    # Same base version: release sorts above pre-release
    else if va.isPreRelease != vb.isPreRelease
    then
      if va.isPreRelease
      then -1
      else 1
    # Both pre-release: higher counter wins
    else va.preNum - vb.preNum;

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
