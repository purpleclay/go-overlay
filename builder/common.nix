# Shared infrastructure used by all four builders:
# - commonRemovedAttrs: builder-owned parameters stripped before passing to stdenv
# - mkCommonAttrs: env setup, build, check, and install phases shared across builders
{lib}: let
  inherit (lib) concatMapStringsSep concatStringsSep escapeShellArg optionalString;
in {
  # Builder-owned parameters stripped from attrs before passing to stdenv.mkDerivation.
  commonRemovedAttrs = [
    "modules"
    "subPackages"
    "ldflags"
    "tags"
    "GOOS"
    "GOARCH"
    "CGO_ENABLED"
    "localReplaces"
    "netrcFile"
    "GOPRIVATE"
    "GONOSUMDB"
    "GONOPROXY"
    "allowGoReference"
    "checkFlags"
    "extraGoFlags"
    "excludedPackages"
    "meta"
  ];

  # Shared derivation attributes used by all builders. Builder-specific pieces
  # (configurePhase, test targets, passthru) are pre-computed and passed in.
  mkCommonAttrs = {
    attrs,
    go,
    allowGoReference,
    ldflags,
    tags,
    GOOS,
    GOARCH,
    CGO_ENABLED,
    useVendor,
    subPackages,
    checkFlags,
    extraGoFlags ? [],
    hostTools ? [],
    testPackages,
    configurePhase,
    passthru,
    GOWORK ? null,
  }: let
    userEnv = attrs.env or {};
    computedGoFlags =
      optionalString useVendor "-mod=vendor"
      + optionalString (!allowGoReference) (optionalString useVendor " " + "-trimpath")
      + optionalString (extraGoFlags != []) (
        optionalString (useVendor || !allowGoReference) " "
        + concatStringsSep " " extraGoFlags
      );
  in
    assert !(extraGoFlags != [] && userEnv ? GOFLAGS)
    || throw "go-overlay: extraGoFlags cannot be combined with attrs.env.GOFLAGS; use one or the other"; {
      meta = attrs.meta or {};

      nativeBuildInputs =
        (attrs.nativeBuildInputs or [])
        ++ [go]
        ++ hostTools;

      env =
        {
          GOFLAGS = computedGoFlags;
          GODEBUG = lib.optionalString (lib.versionAtLeast go.version "1.25") "embedfollowsymlinks=1";
        }
        // userEnv
        // {
          inherit GOOS GOARCH CGO_ENABLED;
          GO111MODULE = "on";
          GOTOOLCHAIN = "local";
          GOPROXY = "off";
        }
        // lib.optionalAttrs (GOWORK != null) {inherit GOWORK;};

      inherit configurePhase;

      strictDeps = true;

      buildPhase = let
        allLdflags =
          if allowGoReference
          then ldflags
          else ["-buildid="] ++ ldflags;
      in
        attrs.buildPhase or ''
          runHook preBuild

          buildFlags=(
            -v
            -p $NIX_BUILD_CORES
            ${optionalString (allLdflags != []) "-ldflags=${escapeShellArg (concatStringsSep " " allLdflags)}"}
            ${optionalString (tags != []) "-tags=${concatStringsSep "," tags}"}
          )

          for pkg in ${concatStringsSep " " subPackages}; do
            echo "Building $pkg"
            go install "''${buildFlags[@]}" "./$pkg"
          done

          runHook postBuild
        '';

      doCheck = attrs.doCheck or true;

      checkPhase =
        attrs.checkPhase or ''
          runHook preCheck

          export GOFLAGS=''${GOFLAGS//-trimpath/}

          go test \
            -v \
            -p $NIX_BUILD_CORES \
            -vet=off \
            ${optionalString (tags != []) "-tags=${concatStringsSep "," tags}"} \
            ${optionalString (checkFlags != []) (concatStringsSep " " checkFlags)} \
            ${testPackages}

          runHook postCheck
        '';

      installPhase =
        attrs.installPhase or ''
          runHook preInstall

          mkdir -p $out
          if [ -d "$GOPATH/bin" ]; then
            cp -r "$GOPATH/bin" $out/
          fi

          runHook postInstall
        '';

      disallowedReferences = lib.optional (!allowGoReference) go;

      passthru = (attrs.passthru or {}) // passthru;
    };
}
