{
  pkgs,
  go,
}: {
  integration-build-go-module = import ./build-go-module {
    inherit pkgs go;
  };
  integration-check-phase = import ./check-phase {
    inherit pkgs go;
  };
  integration-indirect-deps = import ./indirect-deps {
    inherit pkgs go;
  };
  integration-local-replace = import ./local-replace {
    inherit pkgs go;
  };
  integration-local-replace-external = import ./local-replace-external {
    inherit pkgs go;
  };
  integration-stdlib-only = import ./stdlib-only {
    inherit pkgs go;
  };
  integration-tool-directive-codegen = import ./tool-directive-codegen {
    inherit pkgs go;
  };
  integration-workspace-api =
    (import ./workspace {
      inherit pkgs go;
    }).api;
  integration-workspace-worker =
    (import ./workspace {
      inherit pkgs go;
    }).worker;
  integration-workspace-no-gowork = import ./workspace-no-gowork {
    inherit pkgs go;
  };
}
