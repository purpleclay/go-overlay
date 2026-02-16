{pkgs}: let
  go-bin = pkgs.go-bin;

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
  tools-govulncheck-has-known-version = assertEq "tools-govulncheck-has-known-version" true (builtins.hasAttr "1.1.4" go-bin.latest.tools.govulncheck);

  # tools - version selection
  tools-govulncheck-latest-version = assertEq "tools-govulncheck-latest-version" "1.1.4" go-bin.latest.tools.govulncheck.latest.version;
  tools-govulncheck-pinned-version = assertEq "tools-govulncheck-pinned-version" "1.1.3" go-bin.latest.tools.govulncheck."1.1.3".version;

  # tools - binary exists
  tools-govulncheck-has-binary = testBinaryExists "tools-govulncheck-has-binary" go-bin.latest.tools.govulncheck.latest "bin/govulncheck";

  # tools - compatibility (Go 1.21.4 is compatible with govulncheck up to 1.1.3, but not 1.1.4 which requires Go 1.22.0)
  tools-govulncheck-compat-old-go = assertEq "tools-govulncheck-compat-old-go" "1.1.3" go-bin.versions."1.21.4".tools.govulncheck.latest.version;
}
