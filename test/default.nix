{pkgs}: let
  go-bin = pkgs.go-bin;

  # Direct access to tool-manifests internals for ordering assertions
  toolManifests = import ../lib/tool-manifests.nix {lib = pkgs.lib;};

  # Direct access to the testPackages shell-expression builder
  inherit (import ../builder/test-packages.nix {inherit (pkgs) lib;}) mkTestPackages;

  # Returns the index of x in xs, or null if not found.
  # Returning null (rather than a sentinel integer) ensures ordering assertions
  # fail explicitly when an expected version is absent from the list.
  indexOf = xs: x: let
    res =
      builtins.foldl'
      (acc: v:
        if acc.done
        then acc
        else if v == x
        then {
          done = true;
          i = acc.i;
        }
        else {
          done = false;
          i = acc.i + 1;
        })
      {
        done = false;
        i = 0;
      }
      xs;
  in
    if res.done
    then res.i
    else null;

  assertEq = name: expected: actual:
    if expected == actual
    then pkgs.runCommand "test-${name}-pass" {} "touch $out"
    else
      pkgs.runCommand "test-${name}-fail" {} ''
        echo "Test '${name}' failed"
        echo "Expected: ${builtins.toJSON expected}"
        echo "Actual: ${builtins.toJSON actual}"
        exit 1
      '';

  # Runtime test to check if a derivation has a binary at a given path
  testBinaryExists = name: drv: path:
    pkgs.runCommand "test-${name}-binary" {} ''
      if [ -x "${drv}/${path}" ]; then
        touch $out
      else
        echo "Test '${name}' failed: ${path} not found in ${drv}"
        exit 1
      fi
    '';

  # Recursively collects relative file paths under dir, e.g. ["go.mod" "app/go.sum"].
  collectFiles = dir: prefix:
    builtins.concatLists (
      pkgs.lib.mapAttrsToList (
        name: type:
          if type == "directory"
          then collectFiles (dir + "/${name}") "${prefix}${name}/"
          else ["${prefix}${name}"]
      ) (builtins.readDir dir)
    );

  # Asserts every host tool's own src contains exactly expectedFiles,
  # regardless of whatever else exists in the application source
  # (go-overlay#507): go install only ever needs go.mod/go.sum, so each host
  # tool's src must be filtered down to just those rather than inheriting
  # the application's full source tree. Checks every entry in hostTools, not
  # just the first, since toolSrc filtering doesn't vary per tool.
  assertHostToolSrcIsMinimal = name: drv: expectedFiles: let
    expected = builtins.sort (a: b: a < b) expectedFiles;
    actual = map (tool: builtins.sort (a: b: a < b) (collectFiles tool.src "")) drv.hostTools;
  in
    assertEq name (map (_: expected) drv.hostTools) actual;

  # Asserts a host tool's drvPath is identical for two src values that
  # differ only in files outside go.mod/go.sum. This is the property that
  # actually matters for avoiding host tool rebuilds — assertHostToolSrcIsMinimal
  # checks the resulting file set is correct, but doesn't catch a toolSrc
  # construction that still embeds the full src as a derivation input
  # (which keeps the file set minimal yet still busts the store path on
  # every unrelated change — exactly the regression this guards against).
  assertHostToolDrvPathStable = name: mkDrv: srcA: srcB: let
    toolA = builtins.head (mkDrv srcA).hostTools;
    toolB = builtins.head (mkDrv srcB).hostTools;
  in
    assertEq name toolA.drvPath toolB.drvPath;

  # Evaluates the shell expression produced by mkTestPackages and asserts its
  # output matches expected.
  testTestPackages = name: {
    listCmd,
    basePackages,
    excludedPackages,
    expected,
  }: let
    expr = mkTestPackages {inherit listCmd basePackages excludedPackages;};
  in
    pkgs.runCommand "test-${name}" {} ''
      actual="${expr}"
      expected=${pkgs.lib.escapeShellArg expected}
      if [ "$actual" != "$expected" ]; then
        echo "Test '${name}' failed"
        echo "Expected: $expected"
        echo "Actual: $actual"
        exit 1
      fi
      touch $out
    '';
in {
  # latest
  latest-exists = assertEq "latest-exists" true (builtins.stringLength go-bin.latest.version > 0);
  latest-has-go-binary = testBinaryExists "latest-has-go-binary" go-bin.latest "bin/go";

  # latestStable
  latestStable-exists = assertEq "latestStable-exists" true (builtins.stringLength go-bin.latestStable.version > 0);
  latestStable-not-rc = assertEq "latestStable-not-rc" null (builtins.match ".*rc[0-9]+" go-bin.latestStable.version);
  latestStable-has-go-binary = testBinaryExists "latestStable-has-go-binary" go-bin.latestStable "bin/go";

  # versions
  versions-is-attrset = assertEq "versions-is-attrset" true (builtins.isAttrs go-bin.versions);
  versions-not-empty = assertEq "versions-not-empty" true (builtins.length (builtins.attrNames go-bin.versions) > 0);
  versions-contains-known = assertEq "versions-contains-known" true (builtins.hasAttr "1.21.4" go-bin.versions);
  versions-derivation-has-go-binary = testBinaryExists "versions-derivation-has-go-binary" go-bin.versions."1.21.4" "bin/go";

  # hasVersion
  hasVersion-exact = assertEq "hasVersion-exact" true (go-bin.hasVersion "1.21.4");
  hasVersion-partial-fails = assertEq "hasVersion-partial-fails" false (go-bin.hasVersion "1.21");
  hasVersion-nonexistent = assertEq "hasVersion-nonexistent" false (go-bin.hasVersion "1.99.0");
  hasVersion-rc = assertEq "hasVersion-rc" true (go-bin.hasVersion "1.25rc1");

  # isDeprecated
  isDeprecated-old = assertEq "isDeprecated-old" true (go-bin.isDeprecated "1.17.0");
  isDeprecated-rc-not-deprecated = assertEq "isDeprecated-rc-not-deprecated" false (go-bin.isDeprecated "1.25rc1");
  isDeprecated-latest-stable = assertEq "isDeprecated-latest-stable" false (go-bin.isDeprecated go-bin.latestStable.version);

  # fromGoMod
  fromGoMod-with-toolchain = assertEq "fromGoMod-with-toolchain" "1.21.6" (go-bin.fromGoMod ./fixtures/go-with-toolchain.mod).version;
  fromGoMod-exact = assertEq "fromGoMod-exact" "1.21.4" (go-bin.fromGoMod ./fixtures/go-exact.mod).version;
  fromGoMod-minor-only = assertEq "fromGoMod-minor-only" true (pkgs.lib.hasPrefix "1.21." (go-bin.fromGoMod ./fixtures/go-minor-only.mod).version);

  # fromGoModStrict
  fromGoModStrict-with-toolchain = assertEq "fromGoModStrict-with-toolchain" "1.21.6" (go-bin.fromGoModStrict ./fixtures/go-with-toolchain.mod).version;
  fromGoModStrict-exact = assertEq "fromGoModStrict-exact" "1.21.4" (go-bin.fromGoModStrict ./fixtures/go-exact.mod).version;

  # tools - basic attribute structure
  tools-is-attrset = assertEq "tools-is-attrset" true (builtins.isAttrs go-bin.latest.tools);
  tools-has-govulncheck = assertEq "tools-has-govulncheck" true (builtins.hasAttr "govulncheck" go-bin.latest.tools);
  tools-govulncheck-is-attrset = assertEq "tools-govulncheck-is-attrset" true (builtins.isAttrs go-bin.latest.tools.govulncheck);
  tools-govulncheck-has-latest = assertEq "tools-govulncheck-has-latest" true (builtins.hasAttr "latest" go-bin.latest.tools.govulncheck);
  tools-govulncheck-has-known-version = assertEq "tools-govulncheck-has-known-version" true (builtins.hasAttr go-bin.latest.tools.govulncheck.latest.version go-bin.latest.tools.govulncheck);

  # tools - version selection
  tools-govulncheck-latest-version = assertEq "tools-govulncheck-latest-version" true (builtins.isString go-bin.latest.tools.govulncheck.latest.version && go-bin.latest.tools.govulncheck.latest.version != "");
  tools-govulncheck-pinned-version = assertEq "tools-govulncheck-pinned-version" "1.1.3" go-bin.latest.tools.govulncheck."1.1.3".version;

  # tools - binary exists
  tools-govulncheck-has-binary = testBinaryExists "tools-govulncheck-has-binary" go-bin.latest.tools.govulncheck.latest "bin/govulncheck";

  # tools - compatibility (Go 1.21.4 is compatible with govulncheck up to 1.1.3, but not 1.1.4+ which requires Go 1.22.0)
  tools-govulncheck-compat-old-go = assertEq "tools-govulncheck-compat-old-go" "1.1.3" go-bin.versions."1.21.4".tools.govulncheck.latest.version;

  # withTools - bundles toolchain and tools into a single derivation
  withTools-has-go-binary = testBinaryExists "withTools-has-go-binary" (go-bin.latest.withTools ["govulncheck"]) "bin/go";
  withTools-has-tool-binary = testBinaryExists "withTools-has-tool-binary" (go-bin.latest.withTools ["govulncheck"]) "bin/govulncheck";
  withTools-multiple-tools = testBinaryExists "withTools-multiple-tools" (go-bin.latest.withTools ["govulncheck" "gofumpt"]) "bin/gofumpt";

  # withTools - version pinning
  withTools-pinned-version = testBinaryExists "withTools-pinned-version" (go-bin.latest.withTools [
    {
      name = "gofumpt";
      version = "0.7.0";
    }
  ]) "bin/gofumpt";
  withTools-mixed-entries = testBinaryExists "withTools-mixed-entries" (go-bin.latest.withTools [
    "govulncheck"
    {
      name = "gofumpt";
      version = "0.7.0";
    }
  ]) "bin/gofumpt";

  # withDefaultTools - bundles toolchain with curated default tools into a single derivation
  # Binary existence is verified in CI via the build-with-default-tools matrix job
  withDefaultTools-is-derivation = assertEq "withDefaultTools-is-derivation" true (builtins.isAttrs go-bin.latest.withDefaultTools);

  # tools - pre-release version handling (gopls 0.22.0-pre.*)
  # Accessing a pre-release version by its full key must not throw
  tools-gopls-pre-release-accessible = assertEq "tools-gopls-pre-release-accessible" "0.22.0-pre.2" go-bin.latest.tools.gopls."0.22.0-pre.2".version;

  # Higher pre-release counter sorts above lower counter for the same base version
  tools-gopls-pre-release-counter-ordering = let
    sorted = toolManifests.tools.gopls.sortedVersions;
    pre2 = indexOf sorted "0.22.0-pre.2";
    pre1 = indexOf sorted "0.22.0-pre.1";
  in
    assertEq "tools-gopls-pre-release-counter-ordering" true (pre2 != null && pre1 != null && pre2 < pre1);

  # Pre-release of a higher minor version sorts above a stable release of a lower minor version
  tools-gopls-pre-release-above-older-stable = let
    sorted = toolManifests.tools.gopls.sortedVersions;
    pre2 = indexOf sorted "0.22.0-pre.2";
    stable = indexOf sorted "0.21.1";
  in
    assertEq "tools-gopls-pre-release-above-older-stable" true (pre2 != null && stable != null && pre2 < stable);

  # excludedPackages - no exclusions returns basePackages unchanged
  testPackages-no-exclusions = testTestPackages "testPackages-no-exclusions" {
    listCmd = "printf '%s\\n' a ab b";
    basePackages = "./...";
    excludedPackages = [];
    expected = "./...";
  };

  # excludedPackages - exact match only, not substring (excluding "a" must not exclude "ab")
  testPackages-exact-match = testTestPackages "testPackages-exact-match" {
    listCmd = "printf '%s\\n' a ab b";
    basePackages = "./...";
    excludedPackages = ["a"];
    expected = "ab\nb";
  };

  # excludedPackages - excluding every package yields an empty result, not basePackages
  testPackages-all-excluded = testTestPackages "testPackages-all-excluded" {
    listCmd = "printf '%s\\n' a b";
    basePackages = "./...";
    excludedPackages = ["a" "b"];
    expected = "";
  };

  # excludedPackages - a failing listCmd must propagate, not be swallowed into an
  # empty result (which checkPhase would otherwise treat as "all excluded")
  testPackages-listCmd-failure-propagates = let
    expr = mkTestPackages {
      listCmd = "exit 7";
      basePackages = "./...";
      excludedPackages = ["a"];
    };
  in
    pkgs.runCommand "test-testPackages-listCmd-failure-propagates" {} ''
      if (set -e; actual="${expr}"); then
        echo "Test 'testPackages-listCmd-failure-propagates' failed: expected listCmd failure to propagate"
        exit 1
      fi
      touch $out
    '';

  # mkHostTool - a go.mod `tool` directive must produce a working host tool
  # binary for both single-module and workspace builders (go-overlay#507).
  # Regression coverage for decoupling the host tool derivation's src from
  # the application's own source tree.
  hostTool-application-has-binary = let
    drv = pkgs.buildGoApplication {
      pname = "host-tool-app";
      version = "0.1.0";
      src = ./fixtures/host-tool-app;
      go = go-bin.fromGoMod ./fixtures/host-tool-app/go.mod;
    };
  in
    testBinaryExists "hostTool-application-has-binary" (builtins.head drv.hostTools) "bin/stringer";

  hostTool-workspace-has-binary = let
    drv = pkgs.buildGoWorkspace {
      pname = "host-tool-workspace";
      version = "0.1.0";
      src = ./fixtures/host-tool-workspace;
      subPackages = ["app"];
      go = go-bin.fromGoMod ./fixtures/host-tool-workspace/app/go.mod;
    };
  in
    testBinaryExists "hostTool-workspace-has-binary" (builtins.head drv.hostTools) "bin/stringer";

  hostTool-application-src-is-minimal = let
    drv = pkgs.buildGoApplication {
      pname = "host-tool-app";
      version = "0.1.0";
      src = ./fixtures/host-tool-app;
      go = go-bin.fromGoMod ./fixtures/host-tool-app/go.mod;
    };
  in
    assertHostToolSrcIsMinimal "hostTool-application-src-is-minimal" drv ["go.mod" "go.sum"];

  hostTool-workspace-src-is-minimal = let
    drv = pkgs.buildGoWorkspace {
      pname = "host-tool-workspace";
      version = "0.1.0";
      src = ./fixtures/host-tool-workspace;
      subPackages = ["app"];
      go = go-bin.fromGoMod ./fixtures/host-tool-workspace/app/go.mod;
    };
  in
    assertHostToolSrcIsMinimal "hostTool-workspace-src-is-minimal" drv ["app/go.mod" "app/go.sum"];

  # A caller may pass an already-filtered src (e.g. their own
  # lib.fileset.toSource call) rather than a raw path. The minimal-src
  # extraction happens at build time precisely so it isn't limited to
  # literal paths, so this must still produce a minimal src, not just avoid
  # crashing (go-overlay#531).
  hostTool-application-non-path-src-is-minimal = let
    rawSrc = ./fixtures/host-tool-app;
    filteredSrc = pkgs.lib.fileset.toSource {
      root = rawSrc;
      fileset = pkgs.lib.fileset.fromSource (pkgs.lib.sources.cleanSource rawSrc);
    };
    drv = pkgs.buildGoApplication {
      pname = "host-tool-app";
      version = "0.1.0";
      src = filteredSrc;
      go = go-bin.fromGoMod (rawSrc + "/go.mod");
    };
  in
    assertHostToolSrcIsMinimal "hostTool-application-non-path-src-is-minimal" drv ["go.mod" "go.sum"];

  hostTool-application-drvpath-stable-across-unrelated-changes = let
    baseSrc = ./fixtures/host-tool-app;
    touchedSrc =
      pkgs.runCommand "host-tool-app-touched" {}
      ''
        cp -r ${baseSrc} $out
        chmod -R u+w $out
        echo "// unrelated change" >> $out/main.go
      '';
    mkDrv = src:
      pkgs.buildGoApplication {
        pname = "host-tool-app";
        version = "0.1.0";
        inherit src;
        go = go-bin.fromGoMod (baseSrc + "/go.mod");
      };
  in
    assertHostToolDrvPathStable "hostTool-application-drvpath-stable-across-unrelated-changes" mkDrv baseSrc touchedSrc;

  hostTool-workspace-drvpath-stable-across-unrelated-changes = let
    baseSrc = ./fixtures/host-tool-workspace;
    touchedSrc =
      pkgs.runCommand "host-tool-workspace-touched" {}
      ''
        cp -r ${baseSrc} $out
        chmod -R u+w $out
        echo "// unrelated change" >> $out/app/main.go
      '';
    mkDrv = src:
      pkgs.buildGoWorkspace {
        pname = "host-tool-workspace";
        version = "0.1.0";
        inherit src;
        subPackages = ["app"];
        go = go-bin.fromGoMod (baseSrc + "/app/go.mod");
      };
  in
    assertHostToolDrvPathStable "hostTool-workspace-drvpath-stable-across-unrelated-changes" mkDrv baseSrc touchedSrc;
}
