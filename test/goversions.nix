# The goversions CLI, rendered against fixed fixtures so its goldens never
# change with a release, plus checks that it agrees with the overlay.
{pkgs, ...}: let
  inherit (import ./lib.nix {inherit pkgs;}) go-bin toolInfo toolManifests versionInfo versionLib;

  goversionsFixtureRecord = version: date: minor: status: newestInMinor: {
    inherit version date minor status newestInMinor;
    attr = versionLib.packageName version;
  };

  # A fixed stand-in for versionInfo, so goversions goldens don't change with
  # every Go release. Covers every status, an rc ahead of every stable release,
  # a two-digit patch and the widest version string (1.17beta1).
  goversionsFixture = [
    (goversionsFixtureRecord "1.28rc1" "2026-11-12" "1.28" "latest" true)
    (goversionsFixtureRecord "1.27.2" "2026-10-07" "1.27" "stable" true)
    (goversionsFixtureRecord "1.27.1" "2026-09-01" "1.27" "supported" false)
    (goversionsFixtureRecord "1.27rc1" "2026-06-18" "1.27" "prerelease" false)
    (goversionsFixtureRecord "1.26.10" "2026-10-07" "1.26" "supported" true)
    (goversionsFixtureRecord "1.26.9" "2026-09-01" "1.26" "supported" false)
    (goversionsFixtureRecord "1.25.14" "2026-08-13" "1.25" "eol" true)
    (goversionsFixtureRecord "1.25rc2" "2025-07-08" "1.25" "eol" false)
    (goversionsFixtureRecord "1.17beta1" "2021-06-10" "1.17" "eol" true)
  ];
  goversionsToolRecord = tool: version: date: go: license: let
    parts = pkgs.lib.splitString "." version;
  in {
    inherit tool version date go license;
    minor = "${builtins.elemAt parts 0}.${builtins.elemAt parts 1}";
    prerelease = pkgs.lib.hasInfix "-" version;
    goVersion = versionLib.parseVersion go;
  };

  # A fixed stand-in for toolInfo, newest first per tool. gopls's newest
  # release is a pre-release needing an rc of Go, so it's also latest against
  # the fixture's newest Go (1.28rc1). Nothing in gofumpt or gopls works with
  # Go 1.17, so --go 1.17 has tools with no compatible version.
  goversionsToolsFixture = let
    r = goversionsToolRecord;
  in {
    delve = [
      (r "delve" "1.27.2" "2026-08-20" "1.25.0" "MIT")
      (r "delve" "1.26.3" "2026-03-02" "1.24" "MIT")
      (r "delve" "1.21.1" "2023-10-04" "1.17" "MIT")
    ];
    gofumpt = [(r "gofumpt" "0.12.0" "2026-09-11" "1.26.0" "BSD-3-Clause")];
    gopls = [
      (r "gopls" "0.25.0-pre.1" "2026-11-19" "1.28rc1" "BSD-3-Clause")
      (r "gopls" "0.24.0" "2026-10-14" "1.27.0" "BSD-3-Clause")
      (r "gopls" "0.24.0-pre.1" "2026-09-02" "1.27.0" "BSD-3-Clause")
      (r "gopls" "0.23.0" "2026-08-26" "1.26.0" "BSD-3-Clause")
      (r "gopls" "0.21.1" "2026-02-05" "1.25" "BSD-3-Clause")
      (r "gopls" "0.20.0" "2025-07-28" "1.24.2" "BSD-3-Clause")
    ];
  };

  goversionsApp = pkgs.callPackage ../goversions.nix {
    records = goversionsFixture;
    tools = goversionsToolsFixture;
    latestGo = "1.28rc1";
  };

  # Built from the real data, as the flake builds it, to check the CLI agrees
  # with go.tools.<name>.latest
  goversionsRealApp = pkgs.callPackage ../goversions.nix {
    records = versionInfo;
    tools = toolInfo;
    latestGo = go-bin.latest.version;
  };
  goversionsGoldens = ./fixtures/goversions;

  # The other half of the latest/stable split: the newest release is stable,
  # so go-bin.latest and go-bin.latestStable are the same version.
  goversionsStableApp = pkgs.callPackage ../goversions.nix {
    records = [
      (goversionsFixtureRecord "1.27.2" "2026-10-07" "1.27" "latest" true)
      (goversionsFixtureRecord "1.27.1" "2026-09-01" "1.27" "supported" false)
      (goversionsFixtureRecord "1.26.10" "2026-10-07" "1.26" "supported" true)
      (goversionsFixtureRecord "1.25.14" "2026-08-13" "1.25" "eol" true)
    ];
  };

  # Runs goversions against the fixture and checks its exit status, stdout and
  # stderr. stdout is compared with test/fixtures/goversions/<golden>.golden
  # when golden is set; otherwise stdout and stderr are each checked against
  # an expected first line, where "" means nothing may be printed at all.
  #
  # The goldens are rendered from goversionsFixture, so they never change with
  # a Go release. Regenerate one with the fixture-built app each check exposes:
  #   $(nix build --no-link --print-out-paths .#checks.<system>.goversions-default.app)/bin/goversions \
  #     > test/fixtures/goversions/default.golden
  goversionsCase = name: {
    app ? goversionsApp,
    args ? [],
    exit ? 0,
    golden ? null,
    stdout ? "",
    stderr ? "",
  }: let
    expectFirstLine = file: expected:
      if expected == ""
      then ''[ -s ${file} ] && { echo "${file}: expected nothing, got:"; cat ${file}; failed=1; }''
      else ''[ "$(head -n 1 ${file})" = ${pkgs.lib.escapeShellArg expected} ] || { printf '%s: expected first line %s, got:\n' ${file} ${pkgs.lib.escapeShellArg expected}; head -n 1 ${file}; failed=1; }'';
  in
    pkgs.runCommand "test-goversions-${name}" {
      nativeBuildInputs = [pkgs.diffutils];
      passthru = {inherit app;};
    } ''
      set +e
      ${app}/bin/goversions ${pkgs.lib.escapeShellArgs args} >stdout 2>stderr
      status=$?
      set -e
      failed=0
      [ "$status" -eq ${toString exit} ] || { echo "exit status: expected ${toString exit}, got $status"; failed=1; }
      ${
        if golden != null
        then "diff -u ${goversionsGoldens}/${golden}.golden stdout || failed=1"
        else expectFirstLine "stdout" stdout
      }
      ${expectFirstLine "stderr" stderr}
      [ "$failed" -eq 0 ] && touch $out
    '';

  # Runs the renderer directly under gawk and mawk: colour can be forced here
  # without a terminal, and both must produce byte-identical output.
  # Records stdout, stderr and the exit status separately ($out/{stdout,stderr,
  # status}), so a failure that both awks share can't pass as agreement.
  goversionsRenderApp = app: awk: mode: color: prefix:
    pkgs.runCommand "goversions-render-${awk.pname}-${mode}-${toString color}-${
      if prefix == ""
      then "none"
      else prefix
    }" {} ''
      mkdir $out
      set +e
      GOVERSIONS_PREFIX=${pkgs.lib.escapeShellArg prefix} ${awk}/bin/${awk.pname} \
        -v mode=${mode} -v color=${toString color} \
        -f ${../scripts/goversions.awk} ${app.tsv} >$out/stdout 2>$out/stderr
      echo $? >$out/status
    '';
  goversionsRender = goversionsRenderApp goversionsApp;
in {
  # goversions
  goversions-default = goversionsCase "default" {golden = "default";};
  goversions-prefix = goversionsCase "prefix" {
    args = ["1.27"];
    golden = "prefix";
  };
  goversions-all = goversionsCase "all" {
    args = ["--all"];
    golden = "all";
  };
  goversions-help = goversionsCase "help" {
    args = ["--help"];
    golden = "help";
  };
  goversions-version = goversionsCase "version" {
    args = ["--version"];
    stdout = goversionsApp.version;
  };
  goversions-json = goversionsCase "json" {
    args = ["--json"];
    golden = "json";
  };
  goversions-json-prefix = goversionsCase "json-prefix" {
    args = ["--json" "1.27"];
    golden = "json-prefix";
  };
  goversions-json-no-match = goversionsCase "json-no-match" {
    args = ["--json" "1.99"];
    exit = 1;
    stderr = ''goversions: no versions match "1.99"'';
  };
  # The quote would match the one closing the version in the raw JSON line.
  goversions-json-prefix-with-quote = goversionsCase "json-prefix-with-quote" {
    args = ["--json" ''1.27.2"''];
    exit = 1;
    stderr = ''goversions: no versions match "1.27.2""'';
  };
  # The output is one JSON array whose records have exactly the documented
  # keys, version first, which the prefix filter depends on.
  goversions-json-is-an-array = pkgs.runCommand "test-goversions-json-is-an-array" {nativeBuildInputs = [pkgs.jq];} ''
    ${goversionsApp}/bin/goversions --json \
      | jq -e 'type == "array" and length > 0 and all(.[]; keys_unsorted == ["version","date","minor","status","package","newestInMinor"])' \
        >/dev/null && touch $out
  '';
  goversions-no-match = goversionsCase "no-match" {
    args = ["1.99"];
    exit = 1;
    stderr = ''goversions: no versions match "1.99"'';
  };
  # awk's -v would turn this into "1.2" and match; the prefix must be literal.
  goversions-prefix-is-literal = goversionsCase "prefix-is-literal" {
    args = ["1\\.2"];
    exit = 1;
    stderr = ''goversions: no versions match "1\.2"'';
  };
  goversions-bad-flag = goversionsCase "bad-flag" {
    args = ["--bogus"];
    exit = 2;
    stderr = "goversions: unknown option --bogus";
  };
  goversions-two-prefixes = goversionsCase "two-prefixes" {
    args = ["1.27" "1.26"];
    exit = 2;
    stderr = "goversions: only one version prefix may be given";
  };

  # tools
  goversions-tools = goversionsCase "tools" {
    args = ["--tools"];
    golden = "tools";
  };
  goversions-tools-go = goversionsCase "tools-go" {
    args = ["--tools" "--go" "1.17"];
    golden = "tools-go";
  };
  goversions-tools-json = goversionsCase "tools-json" {
    args = ["--tools" "--json" "--go" "1.17"];
    golden = "tools-json";
  };
  goversions-tool = goversionsCase "tool" {
    args = ["gopls"];
    golden = "tool";
  };
  goversions-tool-prefix = goversionsCase "tool-prefix" {
    args = ["gopls" "0.24"];
    golden = "tool-prefix";
  };
  goversions-tool-all = goversionsCase "tool-all" {
    args = ["gopls" "--all"];
    golden = "tool-all";
  };
  goversions-tool-go = goversionsCase "tool-go" {
    args = ["gopls" "--go" "1.26"];
    golden = "tool-go";
  };
  goversions-tool-json = goversionsCase "tool-json" {
    args = ["--json" "gopls"];
    golden = "tool-json";
  };
  goversions-tool-json-go = goversionsCase "tool-json-go" {
    args = ["--json" "gopls" "--go" "1.26"];
    golden = "tool-json-go";
  };
  goversions-tool-no-match = goversionsCase "tool-no-match" {
    args = ["gopls" "0.99"];
    exit = 1;
    stderr = ''goversions: no gopls versions match "0.99"'';
  };
  goversions-tool-none-compatible = goversionsCase "tool-none-compatible" {
    args = ["gopls" "--go" "1.17"];
    exit = 1;
    stderr = "goversions: no gopls versions work with Go 1.17";
  };
  goversions-unknown-tool = goversionsCase "unknown-tool" {
    args = ["goplz"];
    exit = 2;
    stderr = ''goversions: unknown tool "goplz"; valid tools: delve, gofumpt, gopls'';
  };
  goversions-go-without-tool = goversionsCase "go-without-tool" {
    args = ["--go" "1.26"];
    exit = 2;
    stderr = "goversions: --go needs a TOOL or --tools";
  };
  goversions-go-without-version = goversionsCase "go-without-version" {
    args = ["gopls" "--go"];
    exit = 2;
    stderr = "goversions: --go needs a Go version";
  };
  goversions-go-invalid = goversionsCase "go-invalid" {
    args = ["gopls" "--go" "1.x"];
    exit = 2;
    stderr = ''goversions: invalid Go version "1.x"'';
  };
  goversions-tools-with-argument = goversionsCase "tools-with-argument" {
    args = ["--tools" "gopls"];
    exit = 2;
    stderr = "goversions: --tools takes no TOOL or PREFIX";
  };
  goversions-tool-two-prefixes = goversionsCase "tool-two-prefixes" {
    args = ["gopls" "0.24" "0.23"];
    exit = 2;
    stderr = "goversions: only one version prefix may be given";
  };
  goversions-tool-json-shape = pkgs.runCommand "test-goversions-tool-json-shape" {nativeBuildInputs = [pkgs.jq];} ''
    ${goversionsApp}/bin/goversions --json gopls \
      | jq -e 'type == "array" and length > 0 and all(.[]; keys_unsorted == ["tool","version","date","go","license","status","package"])' >/dev/null
    ${goversionsApp}/bin/goversions --json --tools --go 1.17 \
      | jq -e 'type == "array" and length == 3 and all(.[]; keys_unsorted == ["tool","version","date","go","license","package"])' >/dev/null
    touch $out
  '';

  # For every tool and several Go versions, the CLI's latest row must be what
  # go.tools.<name>.latest resolves to, and --tools must agree with it.
  goversions-tools-match-overlay = let
    goVersions = pkgs.lib.unique [go-bin.latest.version go-bin.latestStable.version "1.26.10" "1.21.4"];
    cases = pkgs.lib.cartesianProduct {
      tool = builtins.attrNames toolManifests.tools;
      go = goVersions;
    };
    compare = c: let
      expected = go-bin.versions.${c.go}.tools.${c.tool}.latest.version;
      app = "${goversionsRealApp}/bin/goversions";
    in ''
      got=$(${app} --json ${c.tool} --go ${c.go} | jq -r '.[] | select(.status == "latest") | .version')
      [ "$got" = ${expected} ] || { echo "${c.tool} --go ${c.go}: latest is '$got', overlay resolves ${expected}"; failed=1; }
      got=$(${app} --json --tools --go ${c.go} | jq -r '.[] | select(.tool == "${c.tool}") | .version')
      [ "$got" = ${expected} ] || { echo "--tools --go ${c.go}: ${c.tool} is '$got', overlay resolves ${expected}"; failed=1; }
    '';
    defaultLatest = tool: ''
      got=$(${goversionsRealApp}/bin/goversions --json ${tool} | jq -r '.[] | select(.status == "latest") | .version')
      [ "$got" = ${go-bin.latest.tools.${tool}.latest.version} ] || { echo "${tool}: latest is '$got' without --go"; failed=1; }
    '';
  in
    pkgs.runCommand "test-goversions-tools-match-overlay" {nativeBuildInputs = [pkgs.jq];} ''
      failed=0
      ${pkgs.lib.concatMapStrings compare cases}
      ${pkgs.lib.concatMapStrings defaultLatest (builtins.attrNames toolManifests.tools)}
      [ "$failed" -eq 0 ] && touch $out
    '';
  # Colour needs a terminal through the wrapper, so this renders directly.
  # Regenerate: cp $(nix build --no-link --print-out-paths .#checks.<system>.goversions-colour.render)/stdout test/fixtures/goversions/colour.golden
  goversions-colour = let
    render = goversionsRender pkgs.gawk "all" 1 "";
  in
    pkgs.runCommand "test-goversions-colour" {
      nativeBuildInputs = [pkgs.diffutils];
      passthru = {inherit render;};
    } ''
      [ "$(cat ${render}/status)" = 0 ] || { echo "renderer exited $(cat ${render}/status)"; cat ${render}/stderr; exit 1; }
      diff -u ${goversionsGoldens}/colour.golden ${render}/stdout && touch $out
    '';
  goversions-latest-is-stable = goversionsCase "latest-is-stable" {
    app = goversionsStableApp;
    golden = "latest-is-stable";
  };
  goversions-latest-is-stable-coloured-as-latest = let
    render = goversionsRenderApp goversionsStableApp pkgs.gawk "minor" 1 "";
  in
    pkgs.runCommand "test-goversions-latest-is-stable-coloured-as-latest" {} ''
      # -F: the escape code contains "[", which a regex would misread.
      [ "$(cat ${render}/status)" = 0 ] || { echo "renderer exited $(cat ${render}/status)"; cat ${render}/stderr; exit 1; }
      row=$(grep -F "$(printf '\033[32m')1.27.2 " ${render}/stdout) || { echo "1.27.2 row is not green:"; cat -v ${render}/stdout; exit 1; }
      case "$row" in
        *"latest (stable)"*) touch $out ;;
        *) echo "green row is not labelled latest (stable): $row"; exit 1 ;;
      esac
    '';
  goversions-gawk-and-mawk-agree = let
    cases = pkgs.lib.cartesianProduct {
      mode = ["minor" "all"];
      color = [0 1];
      prefix = ["" "1.2" "1.99"];
    };
    # Agreement alone would let a failure both awks share pass, so each run
    # must also exit as expected: 1 when nothing matches, 0 otherwise.
    compare = c: let
      gawk = goversionsRender pkgs.gawk c.mode c.color c.prefix;
      mawk = goversionsRender pkgs.mawk c.mode c.color c.prefix;
      label = "mode=${c.mode} color=${toString c.color} prefix='${c.prefix}'";
      expected =
        if c.prefix == "1.99"
        then "1"
        else "0";
    in ''
      for part in stdout stderr status; do
        if ! cmp -s ${gawk}/$part ${mawk}/$part; then
          echo "gawk and mawk $part differ: ${label}"; failed=1
        fi
      done
      if [ "$(cat ${gawk}/status)" != ${expected} ]; then
        echo "expected exit ${expected}, got $(cat ${gawk}/status): ${label}"; cat ${gawk}/stderr; failed=1
      fi
    '';
  in
    pkgs.runCommand "test-goversions-gawk-and-mawk-agree" {} ''
      failed=0
      ${pkgs.lib.concatMapStrings compare cases}
      [ "$failed" -eq 0 ] && touch $out
    '';

  # A tool table has no STATUS column, so colour is what marks latest (green)
  # and pre-releases (yellow) on a terminal.
  goversions-tool-latest-and-prerelease-coloured = let
    render = pkgs.runCommand "goversions-tool-colour-render" {} ''
      GOVERSIONS_TOOL=gopls GOVERSIONS_PREFIX="" GOVERSIONS_GO=1.28rc1 \
        ${pkgs.gawk}/bin/gawk -v view=tool -v json=0 -v mode=all -v color=1 -v filter=0 \
        -f ${../scripts/goversions-tools.awk} ${goversionsApp.toolsTsv} >$out
    '';
  in
    pkgs.runCommand "test-goversions-tool-latest-and-prerelease-coloured" {} ''
      # -F: the escape codes contain "[", which a regex would misread.
      grep -qF "$(printf '\033[32m')0.25.0-pre.1 " ${render} || { echo "latest row is not green:"; cat -v ${render}; exit 1; }
      grep -qF "$(printf '\033[33m')0.24.0-pre.1 " ${render} || { echo "pre-release row is not yellow:"; cat -v ${render}; exit 1; }
      grep -qF "$(printf '\033[33m')0.24.0 " ${render} && { echo "stable row is yellow:"; cat -v ${render}; exit 1; }
      touch $out
    '';

  goversions-tools-gawk-and-mawk-agree = let
    cases = pkgs.lib.cartesianProduct {
      view = ["tool" "summary"];
      json = [0 1];
      mode = ["minor" "all"];
      color = [0 1];
      go = ["" "1.26" "1.17"];
    };
    render = awk: c:
      pkgs.runCommand "goversions-tools-render-${awk.pname}" {} ''
        mkdir $out
        set +e
        GOVERSIONS_TOOL=gopls GOVERSIONS_PREFIX="" GOVERSIONS_GO=${
          if c.go == ""
          then "1.28rc1"
          else c.go
        } \
          ${awk}/bin/${awk.pname} -v view=${c.view} -v json=${toString c.json} -v mode=${c.mode} \
          -v color=${toString c.color} -v filter=${
          if c.go == ""
          then "0"
          else "1"
        } \
          -f ${../scripts/goversions-tools.awk} ${goversionsApp.toolsTsv} >$out/stdout 2>$out/stderr
        echo $? >$out/status
      '';
    compare = c: let
      gawk = render pkgs.gawk c;
      mawk = render pkgs.mawk c;
      label = "view=${c.view} json=${toString c.json} mode=${c.mode} color=${toString c.color} go='${c.go}'";
      expected =
        if c.view == "tool" && c.go == "1.17"
        then "1"
        else "0";
    in ''
      for part in stdout stderr status; do
        if ! cmp -s ${gawk}/$part ${mawk}/$part; then
          echo "gawk and mawk $part differ: ${label}"; failed=1
        fi
      done
      if [ "$(cat ${gawk}/status)" != ${expected} ]; then
        echo "expected exit ${expected}, got $(cat ${gawk}/status): ${label}"; cat ${gawk}/stderr; failed=1
      fi
    '';
  in
    pkgs.runCommand "test-goversions-tools-gawk-and-mawk-agree" {} ''
      failed=0
      ${pkgs.lib.concatMapStrings compare cases}
      [ "$failed" -eq 0 ] && touch $out
    '';
}
