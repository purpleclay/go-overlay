# cobra-cli

A Cobra CLI with a bubbletea TUI, demonstrating `subPackages`, `ldflags`, `checkFlags`, and shell completion generation via `postInstall`.

## Getting started

```shell
nix run .#example-cobra-cli -- suggest
```

## The Nix bit

```nix
{
  pkgs,
  go,
}: let
  version = "0.1.0";
in
  pkgs.buildGoApplication {
    inherit go version;

    pname = "doggo";
    src = ./.;
    subPackages = ["cmd/doggo"];
    ldflags = ["-X main.version=${version}"];

    # checkFlags are passed to go test by the default checkPhase, which now
    # tests the entire module (./...) regardless of subPackages.
    checkFlags = [
      "-race" # detect race conditions
      "-shuffle=on" # randomise test execution order to surface order-dependent failures
    ];

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
```
