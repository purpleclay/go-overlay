# Shared logic for computing the `go test` package list when excludedPackages
# is set.
{lib}: let
  inherit (lib) concatMapStringsSep escapeShellArg trim;
in {
  # Build the shell expression used as the test package list.
  #
  # listCmd: the `go list` invocation producing the candidate package paths
  # basePackages: value to use when excludedPackages is empty
  # excludedPackages: exact import paths to exclude from basePackages
  #
  # Excluded paths are matched exactly (grep -Fx) against `go list` output, so
  # excluding "internal/e2e" does not also exclude "internal/e2e2". If every
  # package is excluded, the expression evaluates to an empty string rather
  # than falling back to basePackages.
  #
  # A failure from listCmd itself propagates (the build fails), whereas grep
  # exiting 1 just means every listed package was excluded.
  mkTestPackages = {
    listCmd,
    basePackages,
    excludedPackages,
  }:
    if excludedPackages == []
    then basePackages
    else
      trim ''
        $(
          _pkgs="$(${listCmd})" || exit $?
          printf '%s\n' "$_pkgs" \
            | grep -Fxv ${concatMapStringsSep " " (p: "-e " + escapeShellArg p) excludedPackages} \
            || [ $? -eq 1 ]
        )
      '';
}
