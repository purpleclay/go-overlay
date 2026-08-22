# Shared modules.txt entry/trailer generation for remote (non-local) modules.
# Used by both vendor-env.nix (application builds) and workspace.nix
# (workspace builds), so the two builders can't silently drift out of sync —
# see go-overlay#603 for what happened when workspace.nix's copy of this
# logic fell behind (missing replace trailers, path-equality gating the
# wrong condition).
{lib}: let
  inherit (lib) concatMapStringsSep optionalString;

  # Renders the "## explicit; go X.Y" / "## explicit" / "## go X.Y" annotation
  # line for a remote module, or "" when neither applies.
  mkAnnotation = meta: let
    isImplicit = meta.implicit or false;
    goVersion = meta.go or "";
  in
    if !isImplicit && goVersion != ""
    then "## explicit; go ${goVersion}"
    else if !isImplicit
    then "## explicit"
    else if goVersion != ""
    then "## go ${goVersion}"
    else "";
in {
  inherit mkAnnotation;

  # Whether a module entry is a wildcard remote replace that nothing in the
  # build actually requires (mod.ModuleConfig.Unused). Such an entry has no
  # version, hash, or packages of its own — it must be excluded from both
  # module-entry generation and fetching, and only contributes a trailer line.
  isUnusedReplace = meta: meta.unused or false;

  # Generate the modules.txt header + annotation + packages for a single
  # remote (non-local) module. A module is a remote replace whenever `replaced`
  # is present — regardless of whether the path or only the version changed.
  # `meta.required` carries the version go.mod originally required, when it
  # differs from `meta.version` (the fetched/replacement version); omitted
  # from the manifest, and so absent here, when the two match.
  mkRemoteModuleEntry = goPackagePath: meta: let
    isRemoteReplace = meta ? replaced;
    header =
      if isRemoteReplace
      then "# ${goPackagePath} ${meta.required or meta.version} => ${meta.replaced} ${meta.version}"
      else "# ${goPackagePath} ${meta.version}";
    annotation = mkAnnotation meta;
    packages = concatMapStringsSep "\n" (p: p) (meta.packages or []);
  in
    header
    + optionalString (annotation != "") ("\n" + annotation)
    + optionalString (packages != "") ("\n" + packages);

  # Generate the trailing summary lines that `go mod vendor`/`go work vendor`
  # emit after all module entries, one per remote replace in `modules` (an
  # attrset keyed by module path) that needs one:
  #   - An unversioned (wildcard) replace always gets one — it applies to
  #     every required version, so the inline header alone (when there is
  #     one) isn't a complete summary. Version-less: "# path => replaced version".
  #   - A used, versioned replace gets none — the inline header alone is
  #     unambiguous, and emitting one anyway is what made reconstructed
  #     vendor/modules.txt fail Go's consistency check.
  #   - An unused replace always gets one, since it has no inline header at
  #     all — versioned or not, the trailer is its only representation.
  #     A versioned-and-unused replace's trailer carries the old version on
  #     the left, exactly as a real header would: "# path required => replaced version".
  mkRemoteReplaceTrailers = modules: let
    remoteReplaceModules = lib.filterAttrs (_: meta: (meta ? replaced) && (!(meta.versioned or false) || (meta.unused or false))) modules;
  in
    concatMapStringsSep "\n" (
      goPackagePath: let
        meta = remoteReplaceModules.${goPackagePath};
        isUnusedVersioned = (meta.unused or false) && (meta.versioned or false);
      in
        if isUnusedVersioned
        then "# ${goPackagePath} ${meta.required or meta.version} => ${meta.replaced} ${meta.version}"
        else "# ${goPackagePath} => ${meta.replaced} ${meta.version}"
    ) (builtins.attrNames remoteReplaceModules);
}
