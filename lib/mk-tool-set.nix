# Produces the `tools` attribute set for a specific Go toolchain.
# Each tool exposes all versions as attributes. Compatible versions evaluate to
# a derivation, incompatible versions evaluate to a throw with a helpful message.
# A `latest` attribute resolves to the newest compatible version.
{
  lib,
  buildGoTool,
  toolManifests,
}: let
  inherit (import ./version.nix {inherit lib;}) compareVersions;

  # Returns true if goVersion >= requiredVersion
  isGoCompatible = goVersion: requiredVersion:
    compareVersions goVersion requiredVersion >= 0;
in
  # go: the Go toolchain derivation (has .version attribute)
  go: let
    goVersion = go.version;

    mkToolVersionSet = toolName: toolData: let
      # Falls back to the full manifest when index.nix is missing a version
      # (e.g. stale or not yet regenerated), so a single out-of-sync entry
      # only costs an extra import for that version rather than failing
      # evaluation entirely.
      requiredGoFor = v:
        if builtins.hasAttr v toolData.index
        then toolData.index.${v}.go
        else toolData.manifests.${v}.go;

      versionAttrs =
        lib.mapAttrs (
          version: manifest: let
            compatible = isGoCompatible goVersion manifest.go;

            latestCompatible = builtins.head (
              builtins.filter
              (v: isGoCompatible goVersion (requiredGoFor v))
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
        (v: isGoCompatible goVersion (requiredGoFor v))
        toolData.sortedVersions;

      latestAttr =
        if compatibleVersions == []
        then throw "go-overlay: no version of ${toolName} is compatible with Go ${goVersion}"
        else versionAttrs.${builtins.head compatibleVersions};
    in
      versionAttrs // {latest = latestAttr;};
  in
    lib.mapAttrs mkToolVersionSet toolManifests.tools
