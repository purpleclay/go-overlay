{
  pkgs,
  go,
}: let
  # Auto-discover all subdirectories and generate a simple example-<dir> entry
  # for each. Every subdirectory is expected to expose a default.nix that accepts
  # { pkgs, go }.
  simpleExamples = builtins.listToAttrs (
    map (dir: {
      name = "example-${dir}";
      value = import ./${dir} {inherit pkgs go;};
    })
    (builtins.attrNames (pkgs.lib.filterAttrs (_: v: v == "directory") (builtins.readDir ./.)))
  );
in
  simpleExamples
  // {
    # Variants that pass additional arguments on top of the base directory import.
    example-build-tags-meaningful = import ./build-tags {
      inherit pkgs go;
      tags = ["meaningful"];
    };
    example-build-tags-procrastination = import ./build-tags {
      inherit pkgs go;
      tags = ["procrastination"];
    };
    example-cross-compile-linux = import ./cross-compile {
      inherit pkgs go;
      GOOS = "linux";
      GOARCH = "amd64";
    };
    example-cross-compile-freebsd = import ./cross-compile {
      inherit pkgs go;
      GOOS = "freebsd";
      GOARCH = "amd64";
    };
    example-cross-compile-windows = import ./cross-compile {
      inherit pkgs go;
      GOOS = "windows";
      GOARCH = "amd64";
    };
  }
