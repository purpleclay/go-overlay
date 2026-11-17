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

  # Each module supplies the files in its declared packages. A child replaces
  # parent files in shared package directories, retaining parent-only subpackages.
  mkModuleCopyCommands = {
    sources,
    packages ? {},
    useSymlinks ? true,
  }: let
    # attrNames orders parents before children; reversing it selects the deepest
    # declared ancestor first when looking up a child's parent module.
    pkgPaths = builtins.attrNames sources;
    reversedPaths = lib.reverseList pkgPaths;
    copyFlags =
      if useSymlinks
      then "-s"
      else "--reflink=auto";

    clearParentPackages = packagePaths:
      concatMapStringsSep "\n" (package: ''
        package_dir="$out"/${escapeShellArg package}
        if [ -d "$package_dir" ]; then
          find "$package_dir" -mindepth 1 -maxdepth 1 ! -xtype d -delete
        fi
      '')
      packagePaths;

    prepareParentDirectories = parentModule: packagePaths:
      concatMapStringsSep "\n" (package: let
        parts = lib.splitString "/" (lib.removePrefix "${parentModule}/" package);
      in ''
        parent_dir="$out"/${escapeShellArg parentModule}
        parent_src=${sources.${parentModule}}
        # Source directory links need real output directories before merging.
        for path_part in ${concatMapStringsSep " " escapeShellArg parts}; do
          parent_dir="$parent_dir/$path_part"
          parent_src="$parent_src/$path_part"
          if [ -L "$parent_dir" ]; then
            source_dir=$(readlink -m "$parent_src")
            rm "$parent_dir"
            mkdir "$parent_dir"
            if [ -d "$source_dir" ]; then
              cp -r ${copyFlags} "$source_dir/." "$parent_dir"
              find "$parent_dir" -type d -exec chmod u+w {} +
            fi
          fi
        done
      '')
      packagePaths;
  in
    concatMapStringsSep "\n" (
      goPackagePath: let
        modSrc = sources.${goPackagePath};
        pkgPath = escapeShellArg goPackagePath;
        hasChildren = builtins.any (lib.hasPrefix "${goPackagePath}/") pkgPaths;
        parentModule = lib.findFirst (path: lib.hasPrefix "${path}/" goPackagePath) null reversedPaths;
        modulePackages = packages.${goPackagePath} or [];
      in ''
        pkg_path=${pkgPath}
        ${optionalString (parentModule != null) (
          prepareParentDirectories parentModule (lib.unique ([goPackagePath] ++ modulePackages))
          + clearParentPackages modulePackages
        )}
        ${
          if useSymlinks && !hasChildren && parentModule == null
          then ''
            mkdir -p "$out/$(dirname "$pkg_path")"
            ln -s ${modSrc} "$out/$pkg_path"
          ''
          else ''
            mkdir -p "$out/$pkg_path"
            cp -r --remove-destination ${copyFlags} ${modSrc}/. "$out/$pkg_path/"
            ${optionalString (hasChildren || parentModule != null) ''
              find "$out/$pkg_path" -type d -exec chmod u+w {} +
            ''}
          ''
        }
      ''
    )
    pkgPaths;
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

      ${mkModuleCopyCommands {
        sources = sources // localModuleSources;
        packages = builtins.mapAttrs (_: meta: meta.packages or []) modules;
        inherit useSymlinks;
      }}

      # Write modules.txt
      cp "$modulesTxtPath" "$out/modules.txt"
    '';
}
