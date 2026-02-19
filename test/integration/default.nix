{pkgs}: let
  go_1_22 = pkgs.go-bin.versions."1.22.3";
  go_1_25 = pkgs.go-bin.versions."1.25.0";
in {
  integration-build-go-module = import ./build-go-module {
    inherit pkgs;
    go = go_1_22;
  };
  integration-check-phase = import ./check-phase {
    inherit pkgs;
    go = go_1_22;
  };
  integration-indirect-deps = import ./indirect-deps {
    inherit pkgs;
    go = go_1_22;
  };
  integration-local-replace = import ./local-replace {
    inherit pkgs;
    go = go_1_22;
  };
  integration-local-replace-external = import ./local-replace-external {
    inherit pkgs;
    go = go_1_22;
  };
  integration-stdlib-only = import ./stdlib-only {
    inherit pkgs;
    go = go_1_22;
  };
  integration-tool-directive-codegen = import ./tool-directive-codegen {
    inherit pkgs;
    go = go_1_25;
  };
  integration-workspace-api =
    (import ./workspace {
      inherit pkgs;
      go = go_1_22;
    }).api;
  integration-workspace-worker =
    (import ./workspace {
      inherit pkgs;
      go = go_1_22;
    }).worker;
  integration-workspace-no-gowork = import ./workspace-no-gowork {
    inherit pkgs;
    go = go_1_22;
  };
}
