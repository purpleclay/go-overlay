{
  pkgs,
  go,
}:
pkgs.buildGoApplication {
  inherit go;

  pname = "sqlc-codegen";
  version = "0.1.0";
  src = ./.;
  modules = ./govendor.toml;
  doCheck = false;

  # sqlc is a non-Go tool, so it is provided via nativeBuildInputs rather than
  # the Go tool directive. It is available on $PATH during build phases but is
  # not linked into the output binary — only the generated Go code is.
  nativeBuildInputs = [pkgs.sqlc];

  # sqlc generate runs before the Go compiler, producing the gen/ package from
  # the SQL schema and queries in db/. The gen/ directory is gitignored and
  # exists only inside the Nix sandbox.
  preBuild = ''
    sqlc generate
  '';
}
