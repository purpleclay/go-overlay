# Go tools built from manifests: go-bin's tools attribute set, withTools,
# withDefaultTools and the flake's per-tool packages.
{
  pkgs,
  toolPackages,
  ...
}: let
  inherit (import ./lib.nix {inherit pkgs;}) assertEq go-bin testBinaryExists toolManifests;

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
in {
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

  # manifests/.zed holds editor settings, not a tool
  tools-skips-hidden-directories = assertEq "tools-skips-hidden-directories" [] (builtins.filter (pkgs.lib.hasPrefix ".") (builtins.attrNames toolManifests.tools));

  # The flake exposes one alias per tool, each the latest version built with the latest Go
  tools-flake-aliases-cover-every-tool = assertEq "tools-flake-aliases-cover-every-tool" (builtins.attrNames toolManifests.tools) (builtins.attrNames toolPackages);
  tools-flake-aliases-are-latest = assertEq "tools-flake-aliases-are-latest" [] (builtins.filter (name: !(toolPackages ? ${name}) || toolPackages.${name}.drvPath != go-bin.latest.tools.${name}.latest.drvPath) (builtins.attrNames toolManifests.tools));

  # nix run executes lib.getExe, so meta.mainProgram must name a real binary
  tools-main-program-exists = pkgs.runCommand "test-tools-main-program-exists" {} ''
    failed=0
    ${pkgs.lib.concatMapStringsSep "\n" (name: let
      exe = pkgs.lib.getExe go-bin.latest.tools.${name}.latest;
    in ''
      [ -x ${exe} ] || { echo "${name}: ${exe} is not an executable"; failed=1; }
    '') (builtins.attrNames toolManifests.tools)}
    [ "$failed" -eq 0 ] && touch $out
  '';

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
}
