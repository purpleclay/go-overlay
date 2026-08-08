{lib}: let
  # Parse a Go version string into comparable components.
  # Handles "1.22.0", "1.18", "1.25rc1" and "1.19beta1" formats.
  #
  # stage/counter encoding: beta -> stage = 0; rc -> stage = 1; stable -> stage = 2
  # counter is the rcN/betaN number (0 for stable). Stage and counter are kept
  # as separate fields - rather than collapsed into one band-offset integer -
  # so no counter magnitude can ever cross into another stage's range (a
  # fixed-width band, e.g. rc = -1000 + N, silently collides once N reaches
  # the band's width: 1.19beta1001 and 1.19rc1 both landed on rc = -999).
  parseVersion = v: let
    parts = lib.splitString "." v;
    major = lib.toInt (builtins.elemAt parts 0);
    minorPart = builtins.elemAt parts 1;
    hasRc = builtins.match "([0-9]+)rc([0-9]+)" minorPart;
    hasBeta = builtins.match "([0-9]+)beta([0-9]+)" minorPart;
  in
    if builtins.length parts < 2 || builtins.length parts > 3
    then throw "go-overlay: invalid Go version '${v}' (expected major.minor[.patch][rcN|betaN])"
    else if (hasRc != null || hasBeta != null) && builtins.length parts != 2
    then throw "go-overlay: invalid Go version '${v}' (rcN/betaN versions cannot have a patch component)"
    else if hasRc != null
    then {
      inherit major;
      minor = lib.toInt (builtins.elemAt hasRc 0);
      patch = 0;
      stage = 1;
      counter = lib.toInt (builtins.elemAt hasRc 1);
    }
    else if hasBeta != null
    then {
      inherit major;
      minor = lib.toInt (builtins.elemAt hasBeta 0);
      patch = 0;
      stage = 0;
      counter = lib.toInt (builtins.elemAt hasBeta 1);
    }
    else {
      inherit major;
      minor = lib.toInt minorPart;
      patch =
        if builtins.length parts > 2
        then lib.toInt (builtins.elemAt parts 2)
        else 0;
      stage = 2;
      counter = 0;
    };

  # Compare two version strings; returns a positive int if a > b, negative if
  # a < b, and zero if equal.
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
    else if va.stage != vb.stage
    then va.stage - vb.stage
    else va.counter - vb.counter;
in {
  inherit parseVersion compareVersions;
}
