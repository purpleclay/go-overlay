# The builder: host tools, remote replacements, the go test package list and
# vendor tree construction. The example-* checks join this group in flake.nix.
{pkgs, ...}: let
  inherit (import ./lib.nix {inherit pkgs;}) assertEq go-bin testBinaryExists;

  # Direct access to the testPackages shell-expression builder
  inherit (import ../builder/test-packages.nix {inherit (pkgs) lib;}) mkTestPackages;

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

  # A remote replace directive (replace A => B [version]) must build
  # end-to-end for both builders — regression coverage for go-overlay#603,
  # where the required and replacement versions being conflated into a
  # single manifest field (same-path case) and workspace.nix reconstructing
  # vendor/modules.txt without the replace trailer or fetch-path redirection
  # (differing-path case) both produced a manifest Go's own vendor
  # consistency check rejected at build time.
  remoteReplace-samePath-application-builds = let
    drv = pkgs.buildGoApplication {
      pname = "remote-replace-same-path";
      version = "0.1.0";
      src = ./fixtures/remote-replace-same-path;
      go = go-bin.fromGoMod ./fixtures/remote-replace-same-path/go.mod;
    };
  in
    testBinaryExists "remoteReplace-samePath-application-builds" drv "bin/remote-replace-same-path";

  remoteReplace-differingPath-workspace-builds = let
    drv = pkgs.buildGoWorkspace {
      pname = "app";
      version = "0.1.0";
      src = ./fixtures/workspace-remote-replace;
      subPackages = ["app"];
      go = go-bin.fromGoMod ./fixtures/workspace-remote-replace/app/go.mod;
    };
  in
    testBinaryExists "remoteReplace-differingPath-workspace-builds" drv "bin/app";

  # A used, versioned replace directive (replace A vOld => B vNew, version
  # named on the left) must also build. go mod vendor omits the replace
  # trailer for this form — unlike the wildcard cases above — and always
  # emitting it anyway broke the vendor consistency check the same way
  # omitting it did for the wildcard case (go-overlay#603 follow-up).
  remoteReplace-versioned-application-builds = let
    drv = pkgs.buildGoApplication {
      pname = "remote-replace-versioned";
      version = "0.1.0";
      src = ./fixtures/remote-replace-versioned;
      go = go-bin.fromGoMod ./fixtures/remote-replace-versioned/go.mod;
    };
  in
    testBinaryExists "remoteReplace-versioned-application-builds" drv "bin/remote-replace-versioned";

  # An unused wildcard replace (go.mod declares it, but nothing requires the
  # original path) must build too. go mod vendor records only a trailer for
  # it — no header, hash, or packages — and the replacement target here is
  # independently required by something else, which is what previously made
  # resolution fabricate a bogus header entry for the unused path instead of
  # recognising it as trailer-only (go-overlay#603 follow-up).
  remoteReplace-unused-application-builds = let
    drv = pkgs.buildGoApplication {
      pname = "remote-replace-unused";
      version = "0.1.0";
      src = ./fixtures/remote-replace-unused;
      go = go-bin.fromGoMod ./fixtures/remote-replace-unused/go.mod;
    };
  in
    testBinaryExists "remoteReplace-unused-application-builds" drv "bin/remote-replace-unused";

  # The other unused-replace variant: the replacement target isn't required
  # by anything, used or otherwise, so it's never downloaded at all — the
  # scenario most likely to show up in the wild (a fork replacement whose
  # dependency was later dropped, leaving the replace directive behind).
  remoteReplace-unusedStandalone-application-builds = let
    drv = pkgs.buildGoApplication {
      pname = "remote-replace-unused-standalone";
      version = "0.1.0";
      src = ./fixtures/remote-replace-unused-standalone;
      go = go-bin.fromGoMod ./fixtures/remote-replace-unused-standalone/go.mod;
    };
  in
    testBinaryExists "remoteReplace-unusedStandalone-application-builds" drv "bin/remote-replace-unused-standalone";

  # An unused *versioned* replace (replace A vOld => B vNew, nothing requires
  # A). modules.txt's trailer-only line for this still carries the old
  # version, making it byte-identical in shape to a used, versioned header —
  # this is the case that was silently broken (either a misleading "not
  # found in download output" error, or a fabricated header) before both the
  # detection and the trailer rendering were fixed.
  remoteReplace-unusedVersioned-application-builds = let
    drv = pkgs.buildGoApplication {
      pname = "remote-replace-unused-versioned";
      version = "0.1.0";
      src = ./fixtures/remote-replace-unused-versioned;
      go = go-bin.fromGoMod ./fixtures/remote-replace-unused-versioned/go.mod;
    };
  in
    testBinaryExists "remoteReplace-unusedVersioned-application-builds" drv "bin/remote-replace-unused-versioned";

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

  mkModuleCopyCommands-works-with-tildes-symlink = let
    inherit (import ../builder/modules-txt.nix {inherit (pkgs) lib;}) mkAnnotation mkRemoteModuleEntry mkRemoteReplaceTrailers isUnusedReplace;
    inherit
      (import ../builder/vendor-env.nix {
        inherit (pkgs) lib runCommand fetchGoModule;
        inherit mkAnnotation mkRemoteModuleEntry mkRemoteReplaceTrailers isUnusedReplace;
      })
      mkModuleCopyCommands
      ;

    sources = {
      "git.sr.ht/~sbinet/gg" = ./fixtures/mkModuleCopyCommands-module;
    };
  in
    pkgs.runCommand "test-mkModuleCopyCommands-works-with-tildes-symlink" {} (
      ''
        mkdir -p $out
      ''
      + mkModuleCopyCommands {
        inherit sources;
        useSymlinks = true;
      }
      + ''
        if [ ! -L "$out/git.sr.ht/~sbinet/gg" ]; then
          echo "Test 'mkModuleCopyCommands-works-with-tildes-symlink' failed: git.sr.ht/~sbinet/gg not found in $out"

          exit 1
        fi
      ''
    );

  mkModuleCopyCommands-works-with-tildes-copy = let
    inherit (import ../builder/modules-txt.nix {inherit (pkgs) lib;}) mkAnnotation mkRemoteModuleEntry mkRemoteReplaceTrailers isUnusedReplace;
    inherit
      (import ../builder/vendor-env.nix {
        inherit (pkgs) lib runCommand fetchGoModule;
        inherit mkAnnotation mkRemoteModuleEntry mkRemoteReplaceTrailers isUnusedReplace;
      })
      mkModuleCopyCommands
      ;

    sources = {
      "git.sr.ht/~sbinet/gg" = ./fixtures/mkModuleCopyCommands-module;
    };
  in
    pkgs.runCommand "test-mkModuleCopyCommands-works-with-tildes-copy" {} (
      ''
        mkdir -p $out
      ''
      + mkModuleCopyCommands {
        inherit sources;
        useSymlinks = false;
      }
      + ''
        if [ ! -d "$out/git.sr.ht/~sbinet/gg" ]; then
          echo "Test 'mkModuleCopyCommands-works-with-tildes-copy' failed: git.sr.ht/~sbinet/gg not found in $out"

          exit 1
        fi
      ''
    );
}
