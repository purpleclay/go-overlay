# The flake's checks, one file per group. CI builds each group as its own job,
# run only when the paths it covers change (see .github/workflows/nix-checks.yml),
# so a check belongs to the group of the file it's written in.
#
# packageNames are the flake's versioned package attributes (go_1_27_1, ...),
# so versionInfo's attr can be checked against what the flake really exposes.
# toolPackages are the flake's per-tool aliases (gopls, delve, ...).
{
  pkgs,
  packageNames,
  toolPackages,
}: let
  args = {inherit pkgs packageNames toolPackages;};

  groups = {
    manifests = import ./manifests.nix args;
    goversions = import ./goversions.nix args;
    tools = import ./tools.nix args;
    builder = import ./builder.nix args;
  };

  # The flake merges the groups into one checks set, which would silently keep
  # only one of two checks with the same name.
  names = builtins.concatMap builtins.attrNames (builtins.attrValues groups);
  duplicates = pkgs.lib.unique (builtins.filter (name: pkgs.lib.count (n: n == name) names > 1) names);
in
  if duplicates == []
  then groups
  else throw "go-overlay: checks defined in more than one test file: ${pkgs.lib.concatStringsSep ", " duplicates}"
