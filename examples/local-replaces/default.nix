{
  pkgs,
  go,
}:
pkgs.buildGoApplication {
  inherit go;
  pname = "local-replaces";
  version = "0.1.0";
  src = ./.;
  modules = ./govendor.toml;
  doCheck = false;

  # The go.mod replace directive points to ./units — a relative filesystem path
  # that does not exist inside the Nix sandbox. localReplaces maps each locally
  # replaced module to its Nix store path so the builder can wire it up correctly.
  localReplaces = {
    "github.com/go-overlay/examples/local-replaces/units" = ./units;
  };
}
