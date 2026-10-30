# Metadata for every tool version go-overlay provides, keyed by tool and
# newest first: the order go.tools.<name>.latest resolves from. A pure lookup:
# versions come from manifest filenames and everything else from each tool's
# index.nix, so evaluating it never imports a per-version manifest.
{
  lib,
  toolManifests,
}: let
  inherit (import ./version.nix {inherit lib;}) parseVersion;

  record = name: tool: version: let
    entry = tool.index.${version};
    parts = lib.splitString "." version;
  in {
    inherit version;
    tool = name;
    minor = "${builtins.elemAt parts 0}.${builtins.elemAt parts 1}";
    # e.g. 0.24.0-pre.3 or 0.8.0-rc.1
    prerelease = lib.hasInfix "-" version;
    inherit (entry) date go license;
    # Pre-parsed so goversions can check compatibility at run time with the
    # same rules as go.tools.<name>, without parsing a Go version in awk twice
    goVersion = parseVersion entry.go;
  };
in
  builtins.mapAttrs (name: tool: map (record name tool) tool.sortedVersions) toolManifests.tools
