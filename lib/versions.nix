# Metadata for every Go version go-overlay provides, newest first. A pure
# lookup: versions and ordering come from manifest filenames (manifestsLib)
# and release dates from manifests/go/index.nix, so evaluating it never
# imports a per-version manifest or instantiates a toolchain derivation.
{
  lib,
  manifestsLib,
  index ? import ../manifests/go/index.nix,
}: let
  inherit (import ./version.nix {inherit lib;}) parseVersion packageName;

  minorOf = version: let
    parsed = parseVersion version;
  in "${toString parsed.major}.${toString parsed.minor}";

  # "latest" and "stable" are what go-bin.latest and go-bin.latestStable
  # resolve to, so an rc ahead of every stable release is "latest". "eol"
  # comes before "prerelease" to match isDeprecated, which treats an rc or
  # beta of an end-of-life minor as deprecated. "supported" is any other
  # stable release in a release line Go still supports; the newest in its line
  # is marked by newestInMinor.
  statusOf = version:
    if version == manifestsLib.latest
    then "latest"
    else if version == manifestsLib.latestStable
    then "stable"
    else if manifestsLib.isDeprecated version
    then "eol"
    else if (parseVersion version).stage < 2
    then "prerelease"
    else "supported";

  # sortedVersions is newest first and listToAttrs keeps the first entry for
  # a repeated name, so each minor maps to its newest release.
  newestByMinor = builtins.listToAttrs (map (version: {
      name = minorOf version;
      value = version;
    })
    manifestsLib.sortedVersions);

  dateOf = version:
    index.${version}.date
    or (throw "go-overlay: Go ${version} is missing from manifests/go/index.nix; regenerate it with goscrape go-dev generate");
in
  map (version: {
    inherit version;
    minor = minorOf version;
    date = dateOf version;
    status = statusOf version;
    attr = packageName version;
    newestInMinor = newestByMinor.${minorOf version} == version;
  })
  manifestsLib.sortedVersions
