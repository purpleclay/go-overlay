# Fetches a single Go module using `go mod download`.
# Supports private modules via GOPRIVATE and a netrc file for authentication.
{
  lib,
  stdenvNoCC,
  cacert,
  git,
  jq,
}: {
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
  // lib.optionalAttrs (GOPRIVATE != "") {inherit GOPRIVATE;}
  // lib.optionalAttrs (GONOSUMDB != "") {inherit GONOSUMDB;}
  // lib.optionalAttrs (GONOPROXY != "") {inherit GONOPROXY;}
  // lib.optionalAttrs (netrcFile != null) {
    NETRC_CONTENT = builtins.readFile netrcFile;
  }
)
