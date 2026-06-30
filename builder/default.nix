# Builder entry point — wires sub-modules together and exposes the public API.
#
# Sub-modules (each a focused file):
#   fetch-module.nix  — fetchGoModule (downloads a single Go module)
#   vendor-env.nix    — mkVendorEnv (constructs the vendor/ directory from a manifest)
#   host-tool.nix     — mkHostTool, parseGoWorkModules
#   common.nix        — commonRemovedAttrs, mkCommonAttrs (shared builder infrastructure)
#   test-packages.nix — mkTestPackages (computes the go test package list, honouring excludedPackages)
#   application.nix   — buildGoApplication, buildGoVendoredApplication
#   workspace.nix     — buildGoWorkspace, buildGoVendoredWorkspace
{
  lib,
  stdenv,
  stdenvNoCC,
  runCommand,
  cacert,
  git,
  jq,
}: let
  fetchGoModule = import ./fetch-module.nix {inherit lib stdenvNoCC cacert git jq;};

  vendorEnvModule = import ./vendor-env.nix {inherit lib runCommand fetchGoModule;};
  inherit (vendorEnvModule) mkVendorEnv mkModuleCopyCommands;

  hostToolModule = import ./host-tool.nix {inherit lib stdenv runCommand;};
  inherit (hostToolModule) mkHostTool parseGoWorkModules;

  commonModule = import ./common.nix {inherit lib;};
  inherit (commonModule) commonRemovedAttrs mkCommonAttrs;

  inherit (import ./test-packages.nix {inherit lib;}) mkTestPackages;

  applicationModule = import ./application.nix {
    inherit lib stdenv mkVendorEnv mkHostTool commonRemovedAttrs mkCommonAttrs mkTestPackages;
  };

  workspaceModule = import ./workspace.nix {
    inherit
      lib
      stdenv
      runCommand
      fetchGoModule
      mkModuleCopyCommands
      mkHostTool
      parseGoWorkModules
      commonRemovedAttrs
      mkCommonAttrs
      mkTestPackages
      ;
  };
in {
  inherit fetchGoModule mkVendorEnv;
  inherit (applicationModule) buildGoApplication buildGoVendoredApplication;
  inherit (workspaceModule) buildGoWorkspace buildGoVendoredWorkspace;
}
