{lib}: let
  # Parse a Go version string into comparable components.
  # Handles "1.22.0", "1.18", and "1.25rc1" formats.
  #
  # RC encoding: stable -> rc = 0; rcN -> rc = -1000 + N
  # This keeps all RCs strictly below zero and stable at zero, so the same
  # numeric comparison works for both sorting (compareVersions) and
  # compatibility checks (>= 0 means "at least as new as required").
  parseVersion = v: let
    parts = lib.splitString "." v;
    major = lib.toInt (builtins.elemAt parts 0);
    minorPart = builtins.elemAt parts 1;
    hasRc = builtins.match "([0-9]+)rc([0-9]+)" minorPart;
  in
    if builtins.length parts < 2 || builtins.length parts > 3
    then throw "go-overlay: invalid Go version '${v}' (expected major.minor[.patch][rcN])"
    else if hasRc != null
    then {
      inherit major;
      minor = lib.toInt (builtins.elemAt hasRc 0);
      patch = 0;
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
    else va.rc - vb.rc;
in {
  inherit parseVersion compareVersions;
}
