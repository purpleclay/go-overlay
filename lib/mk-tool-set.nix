# Produces the `tools` attribute set for a specific Go toolchain.
# Each tool exposes all versions as attributes. Compatible versions evaluate to
# a derivation, incompatible versions evaluate to a throw with a helpful message.
# A `latest` attribute resolves to the newest compatible version.
{
  lib,
  buildGoTool,
  toolManifests,
}: let
  # Parse a Go version string into comparable components.
  # Handles "1.22.0", "1.18", and "1.25rc1" formats.
  parseGoVersion = v: let
    parts = lib.splitString "." v;
    major = lib.toInt (builtins.elemAt parts 0);
    minorPart = builtins.elemAt parts 1;
    hasRc = builtins.match "([0-9]+)rc([0-9]+)" minorPart;
  in
    if hasRc != null
    then {
      inherit major;
      minor = lib.toInt (builtins.elemAt hasRc 0);
      patch = 0;
      rc = lib.toInt (builtins.elemAt hasRc 1);
    }
    else {
      inherit major;
      minor = lib.toInt minorPart;
      patch =
        if builtins.length parts > 2
        then lib.toInt (builtins.elemAt parts 2)
        else 0;
      # Stable releases sort after all RCs
      rc = 999999;
    };

  # Returns true if goVersion >= requiredVersion
  isGoCompatible = goVersion: requiredVersion: let
    go = parseGoVersion goVersion;
    req = parseGoVersion requiredVersion;
  in
    if go.major != req.major
    then go.major > req.major
    else if go.minor != req.minor
    then go.minor > req.minor
    else if go.patch != req.patch
    then go.patch > req.patch
    else go.rc >= req.rc;
in
  # go: the Go toolchain derivation (has .version attribute)
  go: let
    goVersion = go.version;

    mkToolVersionSet = toolName: toolData: let
      versionAttrs =
        lib.mapAttrs (
          version: manifest: let
            compatible = isGoCompatible goVersion manifest.go;

            latestCompatible = builtins.head (
              builtins.filter
              (v: isGoCompatible goVersion toolData.manifests.${v}.go)
              toolData.sortedVersions
              ++ ["none"]
            );
            latestCompatibleMsg =
              if latestCompatible == "none"
              then ""
              else "\n\nLatest compatible version: ${latestCompatible}";
          in
            if compatible
            then
              buildGoTool {
                inherit manifest go;
                pname = toolName;
              }
            else
              throw ''
                go-overlay: ${toolName} ${version} requires Go >= ${manifest.go}, but the selected toolchain is Go ${goVersion}.${latestCompatibleMsg}''
        )
        toolData.manifests;

      compatibleVersions =
        builtins.filter
        (v: isGoCompatible goVersion toolData.manifests.${v}.go)
        toolData.sortedVersions;

      latestAttr =
        if compatibleVersions == []
        then throw "go-overlay: no version of ${toolName} is compatible with Go ${goVersion}"
        else versionAttrs.${builtins.head compatibleVersions};
    in
      versionAttrs // {latest = latestAttr;};
  in
    lib.mapAttrs mkToolVersionSet toolManifests.tools
