let
  version = "0.1.0";
  # checkFlags are passed to go test. With a custom checkPhase (see below),
  # thread them through manually; with the default checkPhase they are applied
  # automatically.
  checkFlags = [
    "-race" # detect race conditions
    "-shuffle=on" # randomise test execution order to surface order-dependent failures
    "-vet=off" # disable vet during testing — vet is run as a separate lint step
  ];
in
  {
    pkgs,
    go,
  }:
    pkgs.buildGoApplication {
      inherit go version checkFlags;

      pname = "doggo";
      src = ./.;
      modules = ./govendor.toml;
      subPackages = ["cmd/doggo"];
      ldflags = ["-X main.version=${version}"];

      # subPackages controls what is built, so the default checkPhase would
      # only test ./cmd/... — override it to test the suggest package directly.
      # cmd/doggo is excluded because go:embed requires images at compile time.
      checkPhase = ''
        runHook preCheck
        go test ${builtins.concatStringsSep " " checkFlags} ./internal/suggest/...
        runHook postCheck
      '';

      # Cobra generates shell completions automatically for any CLI built with it.
      # postInstall runs after the binary is installed to $out/bin, so the binary
      # can be invoked here to produce and install the completion files.
      # Plain redirection is used instead of process substitution — the Nix
      # sandbox restricts /dev/fd, which <(...) depends on.
      postInstall = ''
        mkdir -p $out/share/bash-completion/completions
        mkdir -p $out/share/zsh/site-functions
        mkdir -p $out/share/fish/vendor_completions.d
        $out/bin/doggo completion bash > $out/share/bash-completion/completions/doggo
        $out/bin/doggo completion zsh  > $out/share/zsh/site-functions/_doggo
        $out/bin/doggo completion fish > $out/share/fish/vendor_completions.d/doggo.fish
      '';
    }
