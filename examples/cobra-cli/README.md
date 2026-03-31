# cobra-cli

A Cobra CLI with a bubbletea TUI, demonstrating `subPackages`, `ldflags`, `checkFlags`, and shell completion generation via `postInstall`.

## Getting started

```shell
nix run .#example-cobra-cli -- suggest
```

## The Nix bit

```nix
let
  version = "0.1.0";
  # checkFlags are threaded into the custom checkPhase manually.
  checkFlags = [
    "-race"       # detect race conditions
    "-shuffle=on" # randomise test order to surface order-dependent failures
    "-vet=off"    # vet runs separately as a lint step
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

      # The main package lives under cmd/doggo rather than at the module root.
      subPackages = ["cmd/doggo"];

      # Inject the version string at link time.
      ldflags = ["-X main.version=${version}"];

      # subPackages scopes the default checkPhase to ./cmd/... — override it to
      # test internal/suggest directly. cmd/doggo is excluded because go:embed
      # requires the static images to be present at compile time.
      doCheck = true;
      checkPhase = ''
        runHook preCheck
        go test ${builtins.concatStringsSep " " checkFlags} ./internal/suggest/...
        runHook postCheck
      '';

      # Cobra generates shell completions for bash, zsh, and fish automatically.
      # postInstall runs after the binary lands in $out/bin, so the binary can
      # be invoked here to produce and install the completion files.
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
```
