# Constructs a vendor directory with modules.txt from a govendor.toml manifest.
# For Go >= 1.25, modules are symlinked for efficiency (GODEBUG=embedfollowsymlinks=1
# handles //go:embed compatibility). For older Go, modules are copied using
# cp -r --reflink=auto to avoid //go:embed rejecting symlinks as irregular files.
{
  lib,
  runCommand,
  fetchGoModule,
  mkAnnotation,
  mkRemoteModuleEntry,
  mkRemoteReplaceTrailers,
  isUnusedReplace,
}: let
  inherit (lib) concatMapStringsSep escapeShellArg optionalString;

  # Generate modules.txt entry for a single module. Local replacements have
  # no counterpart in workspace.nix's remote/local split, so they stay here;
  # remote entries defer to the shared helper both builders use.
  mkModuleEntry = goPackagePath: meta:
    if meta ? local
    then let
      header = "# ${goPackagePath} ${meta.version} => ${meta.local}";
      annotation = mkAnnotation meta;
      packages = concatMapStringsSep "\n" (p: p) (meta.packages or []);
    in
      header
      + optionalString (annotation != "") ("\n" + annotation)
      + optionalString (packages != "") ("\n" + packages)
    else mkRemoteModuleEntry goPackagePath meta;

  # Generate shell commands to copy fetched modules into $out directory.
  # Handles overlapping module paths by processing deepest paths first and
  # using symlinks where possible for performance.
  mkModuleCopyCommands = {
    sources,
    useSymlinks ? true,
  }: let
    pkgPaths = builtins.attrNames sources;
    # Lexicographic sort puts prefix paths before longer paths within the same domain.
    # Reversing ensures deeper paths (e.g. foo/bar/v2) are placed before their parent
    # (foo/bar), so the symlink for the parent is never created before its children
    # need to write into it.
    pkgPathsSortedByDepth = lib.lists.reverseList (lib.lists.sort (p: q: p < q) pkgPaths);
  in ''
    shopt -s dotglob

    ${concatMapStringsSep "\n" (
        goPackagePath: let
          modSrc = sources.${goPackagePath};

          pkgPath = escapeShellArg goPackagePath;
        in
          if useSymlinks
          then ''
            pkg_path=${pkgPath}

            if [ -d "$out/$pkg_path" ]; then
                cp -rs --update=none ${modSrc}/* "$out/$pkg_path/"
            else
                mkdir -p "$out/$(dirname "$pkg_path")"
                ln -s ${modSrc} "$out/$pkg_path"
            fi
          ''
          else ''
            pkg_path=${pkgPath}

            if [ -d "$out/$pkg_path" ]; then
                cp -r --reflink=auto --update=none ${modSrc}/* "$out/$pkg_path/"
            else
                mkdir -p "$out/$(dirname "$pkg_path")"
                cp -r --reflink=auto ${modSrc} "$out/$pkg_path"
            fi
          ''
      )
      pkgPathsSortedByDepth}

    shopt -u dotglob
  '';
in {
  inherit mkModuleCopyCommands;

  mkVendorEnv = {
    go,
    manifest, # Parsed govendor.toml (via builtins.fromTOML)
    src ? null, # Source tree for local module replacements
    localReplaces ? {}, # Map of module path to Nix path for external local replaces
    netrcFile ? null,
    GOPRIVATE ? "",
    GONOSUMDB ? "",
    GONOPROXY ? "",
  }: let
    useSymlinks = lib.versionAtLeast go.version "1.25";
    modules = manifest.mod or {};

    # Unused entries (mod.ModuleConfig.Unused) have no version, hash, or
    # packages of their own — nothing to fetch, and no header to render, only
    # a trailer (handled by mkRemoteReplaceTrailers against the full `modules`
    # set below, unfiltered).
    remoteModules = lib.filterAttrs (_: meta: !(meta ? local) && !(isUnusedReplace meta)) modules;
    localModules = lib.filterAttrs (_: meta: meta ? local) modules;

    # For remote replacements (replace A => B [version]), govendor hashes the
    # replacement module B, so we must fetch B — not A — to match the stored hash.
    sources =
      builtins.mapAttrs (
        goPackagePath: meta:
          fetchGoModule {
            goPackagePath =
              if meta ? replaced
              then meta.replaced
              else goPackagePath;
            inherit go netrcFile GOPRIVATE GONOSUMDB GONOPROXY;
            inherit (meta) version hash;
          }
      )
      remoteModules;

    modulesTxt = let
      renderedModules = lib.filterAttrs (_: meta: !(isUnusedReplace meta)) modules;
      moduleEntries = concatMapStringsSep "\n" (
        goPackagePath: mkModuleEntry goPackagePath renderedModules.${goPackagePath}
      ) (builtins.attrNames renderedModules);

      localTrailers = concatMapStringsSep "\n" (
        goPackagePath: let
          meta = localModules.${goPackagePath};
        in "# ${goPackagePath} => ${meta.local}"
      ) (builtins.attrNames localModules);

      remoteTrailers = mkRemoteReplaceTrailers modules;
    in
      moduleEntries
      + optionalString (localTrailers != "") ("\n" + localTrailers)
      + optionalString (remoteTrailers != "") ("\n" + remoteTrailers);

    remoteCopyCommands = mkModuleCopyCommands {inherit sources useSymlinks;};

    localModuleSources =
      builtins.mapAttrs (
        goPackagePath: meta:
          if builtins.hasAttr goPackagePath localReplaces
          then localReplaces.${goPackagePath}
          else if src != null
          then "${src}/${meta.local}"
          else throw "go-overlay: Local module '${goPackagePath}' not found in localReplaces and no 'src' provided"
      )
      localModules;
  in
    runCommand "vendor-env"
    {
      passAsFile = ["modulesTxt"];
      inherit modulesTxt;
      passthru = {inherit sources useSymlinks;};
      # Add localReplaces paths as explicit derivation inputs so they're tracked
      # and fetched before the build runs (fixes CI builds where store paths
      # don't exist yet)
      localReplaceSrcs = lib.attrValues localReplaces;
    }
    ''
      mkdir -p $out

      # Copy remote modules
      ${remoteCopyCommands}

      # Copy local modules from source tree
      ${mkModuleCopyCommands {
        sources = localModuleSources;
        inherit useSymlinks;
      }}

      # Write modules.txt
      cp "$modulesTxtPath" "$out/modules.txt"
    '';
}
