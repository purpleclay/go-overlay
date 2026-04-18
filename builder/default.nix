# Builder for Go applications using vendored dependencies.
# Supports three modes:
# 1. In-tree vendor: Use existing vendor/ directory from src
# 2. Manifest mode: Generate vendor from govendor.toml
# 3. Workspace mode: Build from go.work with workspace modules
#
# Unlike gomod2nix, this generates a vendor/modules.txt file so it works
# with unpatched Go toolchains from the official binary distributions.
{
  lib,
  stdenv,
  stdenvNoCC,
  runCommand,
  cacert,
  git,
  jq,
}: let
  inherit
    (builtins)
    fromTOML
    mapAttrs
    readFile
    ;

  inherit
    (lib)
    concatMapStringsSep
    concatStringsSep
    escapeShellArg
    optionalAttrs
    optionalString
    pathExists
    ;

  # Fetch a Go module using `go mod download`.
  # Supports private modules via GOPRIVATE and a netrc file for authentication.
  # Pass netrcFile to provide credentials for private module hosts. The file is
  # copied into the build sandbox's HOME as .netrc so both Go's HTTP client and
  # git (via libcurl) can authenticate.
  fetchGoModule = {
    goPackagePath,
    version,
    hash, # NAR hash from govendor.toml
    go,
    netrcFile ? null, # Path to a .netrc file for private module authentication
    GOPRIVATE ? "", # Comma-separated list of module path prefixes to bypass proxy and checksum DB
    GONOSUMDB ? "", # Comma-separated list of module path prefixes to bypass checksum DB
    GONOPROXY ? "", # Comma-separated list of module path prefixes to bypass proxy
  }:
    stdenvNoCC.mkDerivation (
      {
        name = "${baseNameOf goPackagePath}_${version}";
        builder = ./fetch.sh;
        inherit goPackagePath version;
        nativeBuildInputs = [
          cacert
          git
          go
          jq
        ];
        outputHashMode = "recursive";
        outputHashAlgo = null;
        outputHash = hash;
        impureEnvVars = lib.fetchers.proxyImpureEnvVars ++ ["GOPROXY"];
      }
      // optionalAttrs (GOPRIVATE != "") {inherit GOPRIVATE;}
      // optionalAttrs (GONOSUMDB != "") {inherit GONOSUMDB;}
      // optionalAttrs (GONOPROXY != "") {inherit GONOPROXY;}
      // optionalAttrs (netrcFile != null) {
        NETRC_CONTENT = readFile netrcFile;
      }
    );

  # Generate modules.txt entry for a single module
  mkModuleEntry = goPackagePath: meta: let
    # A remote path replacement has replaced set to a *different* module path.
    # Local replacements also set replaced (to the same path) alongside local.
    isRemoteReplace = (meta ? replaced) && meta.replaced != goPackagePath;
    header =
      if meta ? local
      then "# ${goPackagePath} ${meta.version} => ${meta.local}"
      else if isRemoteReplace
      then "# ${goPackagePath} ${meta.version} => ${meta.replaced} ${meta.version}"
      else "# ${goPackagePath} ${meta.version}";
    explicit =
      if meta.go or "" != ""
      then "## explicit; go ${meta.go}"
      else "## explicit";
    packages = concatMapStringsSep "\n" (p: p) (meta.packages or []);
  in
    header + "\n" + explicit + optionalString (packages != "") ("\n" + packages);

  # Generate shell commands to copy fetched modules into $out directory.
  # Handles overlapping module paths by processing deepest paths first and
  # using symlinks where possible for performance.
  mkModuleCopyCommands = {
    sources,
    useSymlinks ? true,
  }: let
    pkgPaths = builtins.attrNames sources;
    pkgPathsSortedByDepth = lib.lists.reverseList (lib.lists.sort (p: q: p < q) pkgPaths);
  in ''
    shopt -s dotglob

    ${concatMapStringsSep "\n" (
        goPackagePath: let
          modSrc = sources.${goPackagePath};
        in
          if useSymlinks
          then ''
            if [ -d "$out/${escapeShellArg goPackagePath}" ]; then
                cp -rs --update=none ${modSrc}/* "$out/${escapeShellArg goPackagePath}/"
            else
                mkdir -p "$out/$(dirname ${escapeShellArg goPackagePath})"
                ln -s ${modSrc} "$out/${escapeShellArg goPackagePath}"
            fi
          ''
          else ''
            if [ -d "$out/${escapeShellArg goPackagePath}" ]; then
                cp -r --reflink=auto --update=none ${modSrc}/* "$out/${escapeShellArg goPackagePath}/"
            else
                mkdir -p "$out/$(dirname ${escapeShellArg goPackagePath})"
                cp -r --reflink=auto ${modSrc} "$out/${escapeShellArg goPackagePath}"
            fi
          ''
      )
      pkgPathsSortedByDepth}

    shopt -u dotglob
  '';

  # Create a vendor directory with modules.txt from a govendor.toml manifest.
  # For Go >= 1.25, modules are symlinked for efficiency (GODEBUG=embedfollowsymlinks=1
  # handles //go:embed compatibility). For older Go, modules are copied using
  # cp -r --reflink=auto to avoid //go:embed rejecting symlinks as irregular files.
  # For local modules (replace directives with local paths), the source is
  # provided separately and copied during the build phase.
  mkVendorEnv = {
    go,
    manifest, # Parsed govendor.toml (via builtins.fromTOML)
    src ? null, # Source tree for local module replacements
    localReplaces ? {}, # Map of module path to Nix path for external local replaces
    netrcFile ? null, # Path to a .netrc file for private module authentication
    GOPRIVATE ? "", # Comma-separated list of module path prefixes to bypass proxy and checksum DB
    GONOSUMDB ? "", # Comma-separated list of module path prefixes to bypass checksum DB
    GONOPROXY ? "", # Comma-separated list of module path prefixes to bypass proxy
  }: let
    useSymlinks = lib.versionAtLeast go.version "1.25";
    modules = manifest.mod or {};

    # Separate remote and local modules
    remoteModules = lib.filterAttrs (_: meta: !(meta ? local)) modules;
    localModules = lib.filterAttrs (_: meta: meta ? local) modules;

    # Fetch remote modules only.
    # For remote path replacements (replace A => B version), govendor hashes the
    # replacement module B, so we must fetch B — not A — to match the stored hash.
    # The result is still keyed by the original path A so it lands in vendor/A/.
    sources =
      mapAttrs (
        goPackagePath: meta:
          fetchGoModule {
            goPackagePath =
              if (meta ? replaced) && meta.replaced != goPackagePath
              then meta.replaced
              else goPackagePath;
            inherit go netrcFile GOPRIVATE GONOSUMDB GONOPROXY;
            inherit (meta) version hash;
          }
      )
      remoteModules;

    # Generate the complete modules.txt content in Nix
    # Format for regular modules:
    # # module/path version
    # ## explicit; go X.Y
    # package/path1
    #
    # Format for local replacements (Go requires both lines):
    # # module/path version => ./local/path
    # ## explicit; go X.Y
    # package/path1
    # # module/path => ./local/path
    #
    # Format for remote path replacements (Go requires both lines):
    # # module/path version => replacement/path version
    # ## explicit; go X.Y
    # package/path1
    # # module/path => replacement/path version
    modulesTxt = let
      moduleEntries = concatMapStringsSep "\n" (
        goPackagePath: mkModuleEntry goPackagePath modules.${goPackagePath}
      ) (builtins.attrNames modules);

      # Generate trailing replacement markers for local modules
      localTrailers = concatMapStringsSep "\n" (
        goPackagePath: let
          meta = localModules.${goPackagePath};
        in "# ${goPackagePath} => ${meta.local}"
      ) (builtins.attrNames localModules);

      # Generate trailing replacement markers for remote path replacements.
      # A remote replace has replaced set to a *different* module path (no local field).
      remoteReplaceModules = lib.filterAttrs (goPackagePath: meta: (meta ? replaced) && meta.replaced != goPackagePath) modules;
      remoteTrailers = concatMapStringsSep "\n" (
        goPackagePath: let
          meta = remoteReplaceModules.${goPackagePath};
        in "# ${goPackagePath} => ${meta.replaced} ${meta.version}"
      ) (builtins.attrNames remoteReplaceModules);
    in
      moduleEntries
      + optionalString (localTrailers != "") ("\n" + localTrailers)
      + optionalString (remoteTrailers != "") ("\n" + remoteTrailers);

    # Generate copy commands for remote modules
    # (e.g., go.opentelemetry.io/otel and go.opentelemetry.io/otel/trace)
    remoteCopyCommands = mkModuleCopyCommands {inherit sources useSymlinks;};

    # Resolve local module sources - either from localReplaces or src-relative paths
    # This creates the mapping used both for copy commands and as derivation inputs
    localModuleSources =
      mapAttrs (
        goPackagePath: meta:
          if localReplaces ? ${goPackagePath}
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

  # Shared derivation attributes used by both buildGoApplication and buildGoWorkspace.
  # Captures the env setup, build, check, and install phases that are identical
  # between the two builders. Builder-specific pieces (configurePhase, test targets,
  # and passthru) are pre-computed by each builder and passed in.
  mkCommonAttrs = {
    attrs,
    go,
    allowGoReference,
    ldflags,
    tags,
    GOOS,
    GOARCH,
    CGO_ENABLED,
    useVendor,
    subPackages,
    checkFlags,
    testPackages, # pre-computed test target string (differs between app and workspace)
    configurePhase, # builder-specific configure phase (pre-computed with attrs fallback)
    passthru, # builder-specific passthru attrs
  }: {
    meta = attrs.meta or {};

    nativeBuildInputs =
      (attrs.nativeBuildInputs or [])
      ++ [go];

    env =
      attrs.env or {}
      // {
        inherit GOOS GOARCH CGO_ENABLED;

        GO111MODULE = "on";
        GOTOOLCHAIN = "local";
        GOFLAGS =
          optionalString useVendor "-mod=vendor"
          + optionalString (!allowGoReference) (optionalString useVendor " " + "-trimpath");
        GODEBUG = lib.optionalString (lib.versionAtLeast go.version "1.25") "embedfollowsymlinks=1";
      };

    inherit configurePhase;

    strictDeps = true;

    buildPhase = let
      allLdflags =
        if allowGoReference
        then ldflags
        else ["-buildid="] ++ ldflags;
    in
      attrs.buildPhase or ''
        runHook preBuild

        buildFlags=(
          -v
          -p $NIX_BUILD_CORES
          ${optionalString (allLdflags != []) "-ldflags=${escapeShellArg (concatStringsSep " " allLdflags)}"}
          ${optionalString (tags != []) "-tags=${concatStringsSep "," tags}"}
        )

        for pkg in ${concatStringsSep " " subPackages}; do
          echo "Building $pkg"
          go install "''${buildFlags[@]}" "./$pkg"
        done

        runHook postBuild
      '';

    doCheck = attrs.doCheck or false;

    checkPhase =
      attrs.checkPhase or ''
        runHook preCheck

        export GOFLAGS=''${GOFLAGS//-trimpath/}

        go test \
          -v \
          -p $NIX_BUILD_CORES \
          -vet=off \
          ${optionalString (tags != []) "-tags=${concatStringsSep "," tags}"} \
          ${optionalString (checkFlags != []) (concatStringsSep " " checkFlags)} \
          ${testPackages}

        runHook postCheck
      '';

    installPhase =
      attrs.installPhase or ''
        runHook preInstall

        mkdir -p $out
        if [ -d "$GOPATH/bin" ]; then
          cp -r "$GOPATH/bin" $out/
        fi

        runHook postInstall
      '';

    disallowedReferences = lib.optional (!allowGoReference) go;

    passthru = (attrs.passthru or {}) // passthru;
  };

  # Build a Go application using vendored dependencies.
  # Supports two modes:
  # 1. In-tree vendor: If modules is null and src contains vendor/, use it directly
  # 2. Manifest mode: Generate vendor from govendor.toml (modules parameter)
  #
  # Unlike buildGoModule, this works with unpatched Go from binary distributions.
  buildGoApplication = {
    pname,
    version,
    src,
    modules ? null, # Path to govendor.toml manifest (null = auto-detect)
    go, # Go derivation from go-overlay (e.g., go-bin.fromGoMod)
    subPackages ? ["."], # Packages to build (relative to src)
    ldflags ? [],
    tags ? [],
    allowGoReference ? false, # When true, disables -trimpath, -buildid= and disallowedReferences
    localReplaces ? {}, # Map of module path to Nix path for external local replaces
    netrcFile ? null, # Path to a .netrc file for private module authentication
    GOPRIVATE ? "", # Comma-separated list of module path prefixes to bypass proxy and checksum DB
    GONOSUMDB ? "", # Comma-separated list of module path prefixes to bypass checksum DB
    GONOPROXY ? "", # Comma-separated list of module path prefixes to bypass proxy
    checkFlags ? [], # Additional flags passed to go test
    excludedPackages ? [], # Packages to exclude from testing
    CGO_ENABLED ? go.CGO_ENABLED,
    GOOS ? go.GOOS,
    GOARCH ? go.GOARCH,
    ...
  } @ attrs: let
    # Check for in-tree vendor directory
    hasInTreeVendor = pathExists (src + "/vendor");

    # Determine vendor mode
    useInTreeVendor = modules == null && hasInTreeVendor;
    useManifest = modules != null;
    useVendor = useInTreeVendor || useManifest;

    # Guard against missing modules for projects with external dependencies.
    # A go.sum indicates the project depends on external modules and needs
    # either a govendor.toml manifest or an in-tree vendor directory.
    hasGoSum = pathExists (src + "/go.sum");

    # Only parse manifest and create vendorEnv when using manifest mode
    manifest =
      if useManifest
      then
        if pathExists modules
        then fromTOML (readFile modules)
        else
          throw ''
            buildGoApplication: govendor.toml not found at ${toString modules}

              Generate one by running:
                govendor

              Or specify a custom path:
                buildGoApplication {
                  modules = ./path/to/govendor.toml;
                }
          ''
      else null;

    vendorEnv =
      if useManifest
      then
        mkVendorEnv {
          inherit go manifest src localReplaces netrcFile GOPRIVATE GONOSUMDB GONOPROXY;
        }
      else null;

    configurePhase =
      attrs.configurePhase
      or (
        if useInTreeVendor
        then ''
          runHook preConfigure

          export GOCACHE=$TMPDIR/go-cache
          export GOPATH="$TMPDIR/go"
          export GOPROXY=off

          # Use in-tree vendor directory as-is
          chmod -R u+w vendor

          runHook postConfigure
        ''
        else if useManifest
        then ''
          runHook preConfigure

          export GOCACHE=$TMPDIR/go-cache
          export GOPATH="$TMPDIR/go"
          export GOPROXY=off

          # Copy vendor environment from manifest
          rm -rf vendor
          ${
            if vendorEnv.useSymlinks
            then "cp --no-preserve=mode -rs ${vendorEnv} vendor"
            else "cp -r --reflink=auto ${vendorEnv} vendor"
          }
          chmod -R u+w vendor

          runHook postConfigure
        ''
        else ''
          runHook preConfigure

          export GOCACHE=$TMPDIR/go-cache
          export GOPATH="$TMPDIR/go"
          export GOPROXY=off

          runHook postConfigure
        ''
      );

    testPackages =
      if excludedPackages == []
      then concatMapStringsSep " " (p: "./${p}/...") subPackages
      else
        "$(go list ${concatMapStringsSep " " (p: "./${p}/...") subPackages}"
        + " | grep -F -v -- ${concatMapStringsSep " | grep -F -v -- " (p: escapeShellArg p) excludedPackages})";

    passthru = {inherit go vendorEnv;};
  in
    assert (useVendor || !hasGoSum)
    || throw ''
      buildGoApplication: project has external dependencies (go.sum exists)
      but no vendor source was provided.

        Generate a govendor.toml by running:
          govendor

        Then pass it to buildGoApplication:
          buildGoApplication {
            modules = ./govendor.toml;
          }

        Alternatively, commit a vendor directory using 'go mod vendor'.
    '';
      stdenv.mkDerivation (
        builtins.removeAttrs attrs ["modules" "subPackages" "ldflags" "tags" "GOOS" "GOARCH" "CGO_ENABLED" "localReplaces" "netrcFile" "GOPRIVATE" "GONOSUMDB" "GONOPROXY" "allowGoReference" "checkFlags" "excludedPackages" "meta"]
        // {
          inherit pname version src;
        }
        // mkCommonAttrs {
          inherit attrs go allowGoReference ldflags tags GOOS GOARCH CGO_ENABLED;
          inherit useVendor subPackages checkFlags testPackages configurePhase passthru;
        }
      );

  # Build a Go workspace using vendored dependencies.
  # Supports two modes:
  # 1. In-tree vendor: If modules is null and src contains vendor/, use it directly
  # 2. Manifest mode: Generate vendor from govendor.toml (modules parameter)
  #
  # Workspace modules stay in the source tree - only external dependencies are vendored.
  # Workspace module deps (without hash in manifest) are listed in modules.txt but not fetched.
  buildGoWorkspace = {
    pname,
    version,
    src,
    modules ? null, # Path to govendor.toml manifest (null = use in-tree vendor)
    go, # Go derivation from go-overlay
    subPackages ? ["."], # Packages to build (relative to src)
    ldflags ? [],
    tags ? [],
    allowGoReference ? false, # When true, disables -trimpath, -buildid= and disallowedReferences
    localReplaces ? {}, # Map of module path to Nix path for external local replaces
    netrcFile ? null, # Path to a .netrc file for private module authentication
    GOPRIVATE ? "", # Comma-separated list of module path prefixes to bypass proxy and checksum DB
    GONOSUMDB ? "", # Comma-separated list of module path prefixes to bypass checksum DB
    GONOPROXY ? "", # Comma-separated list of module path prefixes to bypass proxy
    checkFlags ? [], # Additional flags passed to go test
    excludedPackages ? [], # Packages to exclude from testing
    CGO_ENABLED ? go.CGO_ENABLED,
    GOOS ? go.GOOS,
    GOARCH ? go.GOARCH,
    ...
  } @ attrs: let
    # Check for in-tree vendor directory
    hasInTreeVendor = pathExists (src + "/vendor");

    # Determine vendor mode
    useInTreeVendor = modules == null && hasInTreeVendor;
    useManifest = modules != null;
    useVendor = useInTreeVendor || useManifest;

    # Guard against missing modules for projects with external dependencies.
    # A go.sum indicates the project depends on external modules and needs
    # either a govendor.toml manifest or an in-tree vendor directory.
    hasGoSum = pathExists (src + "/go.sum");

    # Only parse manifest and create vendorEnv when using manifest mode
    manifest =
      if useManifest
      then
        if pathExists modules
        then fromTOML (readFile modules)
        else
          throw ''
            buildGoWorkspace: govendor.toml not found at ${toString modules}

              Generate one by running:
                govendor

              Or specify a custom path:
                buildGoWorkspace {
                  modules = ./path/to/govendor.toml;
                }
          ''
      else null;

    allModules =
      if manifest != null
      then manifest.mod or {}
      else {};

    workspaceConfig =
      if manifest != null
      then
        if manifest ? workspace
        then manifest.workspace
        else
          throw ''
            buildGoWorkspace: govendor.toml at ${toString modules} has no [workspace] section.

              Regenerate it from your go.work by running:
                govendor
          ''
      else null;

    # Generate go.work content from manifest workspace config
    goWorkContent =
      if workspaceConfig != null
      then
        "go ${workspaceConfig.go}\n"
        + optionalString (workspaceConfig.toolchain or "" != "") "toolchain ${workspaceConfig.toolchain}\n"
        + "\n"
        + "use (\n"
        + concatMapStringsSep "\n" (mod: "\t${mod}") (workspaceConfig.modules or [])
        + "\n)\n"
      else null;

    # Workspace member paths (e.g. ["./mood" "./server"]) — used to distinguish
    # workspace-internal modules from external local replaces in the manifest.
    workspaceMemberPaths =
      if workspaceConfig != null
      then workspaceConfig.modules or []
      else [];

    # Modules with hash are fetched; workspace deps have no hash;
    # local replaces have a local field that is NOT a workspace member path.
    remoteModules = lib.filterAttrs (_: meta: meta ? hash && meta.hash != "" && !(meta ? local)) allModules;
    workspaceDepModules = lib.filterAttrs (_: meta: (!(meta ? hash) || meta.hash == "") && (!(meta ? local) || builtins.elem meta.local workspaceMemberPaths)) allModules;
    localWorkspaceModules = lib.filterAttrs (_: meta: (meta ? local) && !(builtins.elem meta.local workspaceMemberPaths)) allModules;

    # Fetch remote modules only
    externalSources =
      mapAttrs (
        goPackagePath: meta:
          fetchGoModule {
            inherit goPackagePath go netrcFile GOPRIVATE GONOSUMDB GONOPROXY;
            inherit (meta) version hash;
          }
      )
      remoteModules;

    # Resolve local module sources - either from localReplaces or src-relative paths
    localModuleSources =
      mapAttrs (
        goPackagePath: meta:
          if localReplaces ? ${goPackagePath}
          then localReplaces.${goPackagePath}
          else if src != null
          then "${src}/${meta.local}"
          else throw "go-overlay: Local module '${goPackagePath}' not found in localReplaces and no 'src' provided"
      )
      localWorkspaceModules;

    # Generate modules.txt content for workspace
    # Format (from `go work vendor`):
    #   ## workspace
    #   # github.com/workspace/dep v0.1.0
    #   ## explicit; go 1.22
    #   # github.com/external/local-lib v0.0.0 => ../local-lib
    #   ## explicit; go 1.21
    #   # github.com/external/dep v1.0.0
    #   ## explicit; go 1.18
    #   github.com/external/dep
    #   # github.com/external/local-lib => ../local-lib
    modulesTxt = let
      # Workspace dependency entries (no hash, no local field — resolved from source tree)
      workspaceDepEntries = concatMapStringsSep "\n" (
        modPath: let
          meta = workspaceDepModules.${modPath};
          header = "# ${modPath} ${meta.version}";
          explicit =
            if meta.go or "" != ""
            then "## explicit; go ${meta.go}"
            else "## explicit";
        in
          header + "\n" + explicit
      ) (builtins.attrNames workspaceDepModules);

      # Local replace entries (copied into vendor, listed with => ./path format)
      localEntries = concatMapStringsSep "\n" (
        modPath: mkModuleEntry modPath localWorkspaceModules.${modPath}
      ) (builtins.attrNames localWorkspaceModules);

      # Trailing replacement markers for local modules (required by Go toolchain)
      localTrailers = concatMapStringsSep "\n" (
        modPath: let
          meta = localWorkspaceModules.${modPath};
        in "# ${modPath} => ${meta.local}"
      ) (builtins.attrNames localWorkspaceModules);

      # Remote module entries (with hash, fetched from registry)
      remoteEntries = concatMapStringsSep "\n" (
        goPackagePath: mkModuleEntry goPackagePath remoteModules.${goPackagePath}
      ) (builtins.attrNames remoteModules);
    in
      "## workspace\n"
      + workspaceDepEntries
      + optionalString (localEntries != "") ("\n" + localEntries)
      + optionalString (remoteEntries != "") ("\n" + remoteEntries)
      + optionalString (localTrailers != "") ("\n" + localTrailers);

    # Create vendor environment with remote and local deps
    # Workspace module deps are not copied - they stay in the source tree
    useSymlinks = lib.versionAtLeast go.version "1.25";

    vendorEnv =
      if useManifest
      then
        (runCommand "workspace-vendor-env" {
            passAsFile = ["modulesTxt"];
            inherit modulesTxt;
            localReplaceSrcs = lib.attrValues localReplaces;
          } (
            ''
              mkdir -p $out
            ''
            + mkModuleCopyCommands {
              sources = externalSources;
              inherit useSymlinks;
            }
            + mkModuleCopyCommands {
              sources = localModuleSources;
              inherit useSymlinks;
            }
            + ''

              # Write modules.txt
              cp "$modulesTxtPath" "$out/modules.txt"
            ''
          ))
        .overrideAttrs (_: {passthru = {inherit useSymlinks;};})
      else null;

    configurePhase =
      attrs.configurePhase
      or (
        if useInTreeVendor
        then ''
          runHook preConfigure

          export GOCACHE=$TMPDIR/go-cache
          export GOPATH="$TMPDIR/go"
          export GOPROXY=off

          echo "go-overlay: using committed vendor/ directory"

          # Use in-tree vendor directory as-is (from 'go work vendor')
          chmod -R u+w vendor

          runHook postConfigure
        ''
        else if useManifest
        then ''
          runHook preConfigure

          export GOCACHE=$TMPDIR/go-cache
          export GOPATH="$TMPDIR/go"
          export GOPROXY=off

          # Generate go.work if not present in source
          if [ ! -f go.work ]; then
            echo "go-overlay: generating go.work from govendor.toml"
            printf '%s' ${escapeShellArg goWorkContent} > go.work
          else
            echo "go-overlay: using go.work from source tree"
          fi

          # Copy vendor environment with external deps
          # Workspace modules stay in source tree - Go resolves them via modules.txt
          rm -rf vendor
          ${
            if vendorEnv.useSymlinks
            then "cp --no-preserve=mode -rs ${vendorEnv} vendor"
            else "cp -r --reflink=auto ${vendorEnv} vendor"
          }
          chmod -R u+w vendor

          runHook postConfigure
        ''
        else ''
          runHook preConfigure

          export GOCACHE=$TMPDIR/go-cache
          export GOPATH="$TMPDIR/go"
          export GOPROXY=off

          runHook postConfigure
        ''
      );

    # In workspace mode, go test ./... does not expand across module boundaries.
    # Instead, derive test targets from the workspace modules listed in go.work.
    # Falls back to subPackages if no workspace config is present (in-tree vendor mode).
    workspaceTestTargets =
      if workspaceConfig != null
      then concatMapStringsSep " " (mod: "${mod}/...") (workspaceConfig.modules or [])
      else concatMapStringsSep " " (p: "./${p}/...") subPackages;

    testPackages =
      if excludedPackages == []
      then workspaceTestTargets
      else "$(go list ${workspaceTestTargets} | grep -F -v -- ${concatMapStringsSep " | grep -F -v -- " (p: escapeShellArg p) excludedPackages})";

    passthru = {inherit go vendorEnv workspaceConfig;};
  in
    assert (useVendor || !hasGoSum)
    || throw ''
      buildGoWorkspace: project has external dependencies (go.sum exists)
      but no vendor source was provided.

        Generate a govendor.toml by running:
          govendor

        Then pass it to buildGoWorkspace:
          buildGoWorkspace {
            modules = ./govendor.toml;
          }

        Alternatively, commit a vendor directory using 'go work vendor'.
    '';
      stdenv.mkDerivation (
        builtins.removeAttrs attrs ["modules" "subPackages" "ldflags" "tags" "GOOS" "GOARCH" "CGO_ENABLED" "localReplaces" "netrcFile" "GOPRIVATE" "GONOSUMDB" "GONOPROXY" "allowGoReference" "checkFlags" "excludedPackages" "meta"]
        // {
          inherit pname version src;
        }
        // mkCommonAttrs {
          inherit attrs go allowGoReference ldflags tags GOOS GOARCH CGO_ENABLED;
          inherit useVendor subPackages checkFlags testPackages configurePhase passthru;
        }
      );
in {
  inherit buildGoApplication buildGoWorkspace mkVendorEnv fetchGoModule;
}
