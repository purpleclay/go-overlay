# Helpers shared by more than one group of checks.
{pkgs}: let
  go-bin = pkgs.go-bin;

  # Direct access to tool-manifests internals for ordering assertions
  toolManifests = import ../lib/tool-manifests.nix {lib = pkgs.lib;};

  # Direct access to version parsing/comparison internals
  versionLib = import ../lib/version.nix {lib = pkgs.lib;};

  versionInfo = go-bin.versionInfo;

  toolInfo = go-bin.toolInfo;

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
  inherit go-bin toolManifests versionLib versionInfo toolInfo assertEq testBinaryExists;
}
