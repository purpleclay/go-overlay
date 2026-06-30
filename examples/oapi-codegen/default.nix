{
  pkgs,
  go,
}: let
  inherit (pkgs) lib;
in
  pkgs.buildGoApplication {
    inherit go;

    pname = "oapi-codegen";
    version = "0.1.0";

    # Pre-filtering src with lib.fileset is the recommended way to keep docs
    # and other non-build files from busting your application's build cache
    # — the same principle go-overlay applies internally to decouple the
    # host tool's own build from your application source.
    src = lib.fileset.toSource {
      root = ./.;
      fileset = lib.fileset.difference ./. (
        lib.fileset.unions [
          ./README.md
          ./.gitignore
        ]
      );
    };

    # oapi-codegen is declared as a tool directive in go.mod. govendor compiles
    # it for the host platform and injects the binary into nativeBuildInputs,
    # making it available in $PATH here without any extra configuration.
    preBuild = ''
      oapi-codegen --config=api/oapi-codegen.yaml api/catto.yaml
    '';
  }
