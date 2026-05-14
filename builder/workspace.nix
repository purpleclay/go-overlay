# Go workspace builders.
# buildGoWorkspace         — requires a govendor.toml manifest
# buildGoVendoredWorkspace — uses a committed vendor/ directory (go work vendor)
{
  lib,
  stdenv,
  runCommand,
  fetchGoModule,
  mkModuleCopyCommands,
  mkHostTool,
  parseGoWorkModules,
  commonRemovedAttrs,
  mkCommonAttrs,
}: let
  inherit (builtins) fromTOML readFile;
  inherit (lib) concatMapStringsSep escapeShellArg optionalString pathExists;

  # Generate modules.txt entry for a workspace module.
  # Workspace deps have no hash — they are resolved from the source tree.
  mkWorkspaceDepEntry = modPath: meta: let
    header = "# ${modPath} ${meta.version}";
    explicit =
      if meta.go or "" != ""
      then "## explicit; go ${meta.go}"
      else "## explicit";
  in
    header + "\n" + explicit;

  # Generate modules.txt entry for a local-replace module (=> ./path format).
  mkLocalEntry = modPath: meta: let
    header = "# ${modPath} ${meta.version} => ${meta.local}";
    explicit =
      if meta.go or "" != ""
      then "## explicit; go ${meta.go}"
      else "## explicit";
    packages = concatMapStringsSep "\n" (p: p) (meta.packages or []);
  in
    header + "\n" + explicit + optionalString (packages != "") ("\n" + packages);

  # Generate modules.txt entry for a remote module (standard format).
  mkRemoteEntry = modPath: meta: let
    isRemoteReplace = (meta ? replaced) && meta.replaced != modPath;
    header =
      if isRemoteReplace
      then "# ${modPath} ${meta.version} => ${meta.replaced} ${meta.version}"
      else "# ${modPath} ${meta.version}";
    explicit =
      if meta.go or "" != ""
      then "## explicit; go ${meta.go}"
      else "## explicit";
    packages = concatMapStringsSep "\n" (p: p) (meta.packages or []);
  in
    header + "\n" + explicit + optionalString (packages != "") ("\n" + packages);
in {
  # Build a Go workspace from a govendor.toml manifest.
  # Defaults to src + "/govendor.toml" — run `govendor` to generate it.
  # Workspace modules stay in the source tree — only external dependencies are vendored.
  # For projects using `go work vendor`, use buildGoVendoredWorkspace instead.
  buildGoWorkspace = {
    pname,
    version,
    src,
    modules ? src + "/govendor.toml", # Path to govendor.toml manifest
    go,
    subPackages ? ["."],
    ldflags ? [],
    tags ? [],
    allowGoReference ? false,
    localReplaces ? {},
    netrcFile ? null,
    GOPRIVATE ? "",
    GONOSUMDB ? "",
    GONOPROXY ? "",
    checkFlags ? [],
    extraGoFlags ? [],
    excludedPackages ? [],
    CGO_ENABLED ? go.CGO_ENABLED,
    GOOS ? go.GOOS,
    GOARCH ? go.GOARCH,
    ...
  } @ attrs: let
    manifest =
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
        '';

    allModules = manifest.mod or {};

    workspaceConfig =
      if manifest ? workspace
      then manifest.workspace
      else
        throw ''
          buildGoWorkspace: govendor.toml at ${toString modules} has no [workspace] section.

            Regenerate it from your go.work by running:
              govendor
        '';

    goWorkContent =
      "go ${workspaceConfig.go}\n"
      + optionalString (workspaceConfig.toolchain or "" != "") "toolchain ${workspaceConfig.toolchain}\n"
      + "\n"
      + "use (\n"
      + concatMapStringsSep "\n" (mod: "\t${mod}") (workspaceConfig.modules or [])
      + "\n)\n";

    # Workspace member paths — used to distinguish workspace-internal modules
    # from external local replaces in the manifest.
    workspaceMemberPaths = workspaceConfig.modules or [];

    # Modules with hash are fetched; workspace deps have no hash;
    # local replaces have a local field that is NOT a workspace member path.
    remoteModules = lib.filterAttrs (_: meta: meta ? hash && meta.hash != "" && !(meta ? local)) allModules;
    workspaceDepModules = lib.filterAttrs (_: meta: (!(meta ? hash) || meta.hash == "") && (!(meta ? local) || builtins.elem meta.local workspaceMemberPaths)) allModules;
    localWorkspaceModules = lib.filterAttrs (_: meta: (meta ? local) && !(builtins.elem meta.local workspaceMemberPaths)) allModules;

    externalSources =
      builtins.mapAttrs (
        goPackagePath: meta:
          fetchGoModule {
            inherit goPackagePath go netrcFile GOPRIVATE GONOSUMDB GONOPROXY;
            inherit (meta) version hash;
          }
      )
      remoteModules;

    localModuleSources =
      builtins.mapAttrs (
        goPackagePath: meta:
          if localReplaces ? ${goPackagePath}
          then localReplaces.${goPackagePath}
          else "${src}/${meta.local}"
      )
      localWorkspaceModules;

    # Generate modules.txt content for workspace.
    # Format (from `go work vendor`):
    #   ## workspace
    #   # github.com/workspace/dep v0.1.0
    #   ## explicit; go 1.22
    #   # github.com/external/dep v1.0.0
    #   ## explicit; go 1.18
    #   github.com/external/dep
    #   # github.com/external/local-lib => ../local-lib
    modulesTxt =
      "## workspace\n"
      + concatMapStringsSep "\n" (p: mkWorkspaceDepEntry p workspaceDepModules.${p}) (builtins.attrNames workspaceDepModules)
      + optionalString (localWorkspaceModules != {}) (
        "\n"
        + concatMapStringsSep "\n" (p: mkLocalEntry p localWorkspaceModules.${p}) (builtins.attrNames localWorkspaceModules)
      )
      + optionalString (remoteModules != {}) (
        "\n"
        + concatMapStringsSep "\n" (p: mkRemoteEntry p remoteModules.${p}) (builtins.attrNames remoteModules)
      )
      + optionalString (localWorkspaceModules != {}) (
        "\n"
        + concatMapStringsSep "\n" (
          p: "# ${p} => ${localWorkspaceModules.${p}.local}"
        ) (builtins.attrNames localWorkspaceModules)
      );

    useSymlinks = lib.versionAtLeast go.version "1.25";

    vendorEnv = (runCommand "workspace-vendor-env" {
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
      .overrideAttrs (_: {passthru = {inherit useSymlinks;};});

    configurePhase =
      attrs.configurePhase or ''
        runHook preConfigure

        export GOCACHE=$TMPDIR/go-cache
        export GOPATH="$TMPDIR/go"

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
      '';

    # In workspace mode, go test ./... does not expand across module boundaries.
    # Derive test targets from the workspace member paths recorded in the manifest.
    workspaceTestTargets =
      concatMapStringsSep " " (mod: "${mod}/...") (workspaceConfig.modules or []);

    testPackages =
      if excludedPackages == []
      then workspaceTestTargets
      else "$(go list ${workspaceTestTargets} | grep -F -v -- ${concatMapStringsSep " | grep -F -v -- " (p: escapeShellArg p) excludedPackages} || echo ${escapeShellArg workspaceTestTargets})";

    hostTools = map (pkg:
      mkHostTool {
        inherit src go pkg goWorkContent;
        inherit (vendorEnv) useSymlinks;
        vendorEnv = vendorEnv;
        version = manifest.tool.${pkg}.version;
      })
    (builtins.attrNames (manifest.tool or {}));

    passthru = {inherit go vendorEnv workspaceConfig;};
  in
    assert lib.versionAtLeast go.version "1.22"
    || throw ''
      buildGoWorkspace: vendoring in workspace mode requires Go 1.22 or later.

      Go ${go.version} does not support -mod=vendor when a go.work file is present.

        Upgrade to Go 1.22 or later.
    '';
      stdenv.mkDerivation (
        builtins.removeAttrs attrs commonRemovedAttrs
        // {inherit pname version src;}
        // mkCommonAttrs {
          inherit attrs go allowGoReference ldflags tags GOOS GOARCH CGO_ENABLED hostTools;
          inherit subPackages checkFlags extraGoFlags testPackages configurePhase passthru;
          useVendor = true;
        }
      );

  # Build a Go workspace using an in-tree vendor/ directory committed via
  # `go work vendor`. No govendor.toml is required.
  # Unlike buildGoWorkspace, this builder does not provide drift detection,
  # per-dependency hash verification, or Go module tool directive injection.
  buildGoVendoredWorkspace = {
    pname,
    version,
    src,
    go,
    subPackages ? ["."],
    ldflags ? [],
    tags ? [],
    allowGoReference ? false,
    checkFlags ? [],
    extraGoFlags ? [],
    excludedPackages ? [],
    CGO_ENABLED ? go.CGO_ENABLED,
    GOOS ? go.GOOS,
    GOARCH ? go.GOARCH,
    ...
  } @ attrs:
    if !pathExists (src + "/vendor")
    then
      throw ''
        buildGoVendoredWorkspace: no vendor/ directory found in src.

          Commit a vendor directory by running:
            go work vendor
      ''
    else if !pathExists (src + "/go.work")
    then
      throw ''
        buildGoVendoredWorkspace: no go.work file found in src.

          buildGoVendoredWorkspace requires a Go workspace. Create one with:
            go work init
      ''
    else
      assert lib.versionAtLeast go.version "1.22"
      || throw ''
        buildGoVendoredWorkspace: vendoring in workspace mode requires Go 1.22 or later.

        Go ${go.version} does not support -mod=vendor when a go.work file is present.

          Upgrade to Go 1.22 or later.
      ''; let
        workspaceModules = parseGoWorkModules (readFile (src + "/go.work"));

        workspaceTestTargets =
          if workspaceModules != []
          then concatMapStringsSep " " (mod: "${mod}/...") workspaceModules
          else "./...";

        testPackages =
          if excludedPackages == []
          then workspaceTestTargets
          else "$(go list ${workspaceTestTargets} | grep -F -v -- ${concatMapStringsSep " | grep -F -v -- " (p: escapeShellArg p) excludedPackages} || echo ${escapeShellArg workspaceTestTargets})";

        configurePhase =
          attrs.configurePhase or ''
            runHook preConfigure

            export GOCACHE=$TMPDIR/go-cache
            export GOPATH="$TMPDIR/go"

            chmod -R u+w vendor

            runHook postConfigure
          '';

        passthru = {inherit go workspaceModules;};
      in
        stdenv.mkDerivation (
          builtins.removeAttrs attrs commonRemovedAttrs
          // {inherit pname version src;}
          // mkCommonAttrs {
            inherit attrs go allowGoReference ldflags tags GOOS GOARCH CGO_ENABLED;
            inherit subPackages checkFlags extraGoFlags testPackages configurePhase passthru;
            useVendor = true;
            hostTools = [];
          }
        );
}
