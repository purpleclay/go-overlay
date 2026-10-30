# Builds the `goversions` CLI from versionInfo records and toolInfo.
#
# Everything is resolved at evaluation time and written into the store as
# TSVs that scripts/goversions.awk (Go) and scripts/goversions-tools.awk
# (tools) render. The only runtime inputs are the user's arguments, so
# running it needs bash and gawk and compiles nothing. records, tools and
# latestGo are parameters so tests can build the app against fixed fixtures
# rather than the real, ever-growing version lists.
{
  lib,
  writeText,
  writeShellApplication,
  gawk,
  records,
  # toolInfo: tool name -> records, newest first
  tools ? {},
  # The Go version a tool's "latest" is resolved against without --go:
  # go-bin.latest, which the per-tool flake packages are built with
  latestGo ? "",
}: let
  pname = "goversions";
  version = "v0.3.0";

  inherit (import ./lib/version.nix {inherit lib;}) parseVersion;

  # A row shows one status, so when go-bin.latest is a stable release, it is
  # also what go-bin.latestStable resolves to and no row would otherwise say
  # "stable". The combined label is display-only: versionInfo's status stays a
  # single value for scripts to filter on.
  displayStatus = r:
    if r.status == "latest" && (parseVersion r.version).stage == 2
    then "latest (stable)"
    else r.status;

  # 1 version  2 released  3 minor  4 status  5 package  6 newest-in-minor
  tsvRow = r:
    lib.concatStringsSep "\t" [
      r.version
      r.date
      r.minor
      (displayStatus r)
      r.attr
      (
        if r.newestInMinor
        then "1"
        else "0"
      )
    ];

  versionsTsv = writeText "${pname}.tsv" (lib.concatMapStrings (r: tsvRow r + "\n") records);

  # Assembled by hand because builtins.toJSON sorts attrset keys, and --json's
  # prefix filter relies on "version" being the first key. status is
  # versionInfo's own value, never the table's combined "latest (stable)".
  jsonRow = r: let
    j = builtins.toJSON;
  in ''{"version":${j r.version},"date":${j r.date},"minor":${j r.minor},"status":${j r.status},"package":${j r.attr},"newestInMinor":${j r.newestInMinor}}'';

  versionsJsonl = writeText "${pname}.jsonl" (lib.concatMapStrings (r: jsonRow r + "\n") records);

  # 1 tool  2 version  3 released  4 minor  5 prerelease  6 min go  7 license
  # 8-12 min go as major, minor, patch, stage, counter
  toolRow = r:
    lib.concatStringsSep "\t" ([
        r.tool
        r.version
        r.date
        r.minor
        (
          if r.prerelease
          then "1"
          else "0"
        )
        r.go
        r.license
      ]
      ++ map toString [r.goVersion.major r.goVersion.minor r.goVersion.patch r.goVersion.stage r.goVersion.counter]);

  toolNames = builtins.attrNames tools;
  toolsTsv = writeText "${pname}-tools.tsv" (lib.concatMapStrings (r: toolRow r + "\n") (lib.concatMap (name: tools.${name}) toolNames));
in
  writeShellApplication {
    name = pname;
    runtimeInputs = [gawk];

    # Inside a Nix '' string, ${...} is Nix interpolation: shell expansions that
    # use braces must be written ''${...}. Bare $1, $# and "$tsv" need no escaping.
    text = ''
      tsv="${versionsTsv}"
      jsonl="${versionsJsonl}"
      renderer="${./scripts/goversions.awk}"
      tools_tsv="${toolsTsv}"
      tools_renderer="${./scripts/goversions-tools.awk}"
      tool_names="${lib.concatStringsSep " " toolNames}"
      latest_go="${latestGo}"

      # Whether to colour output written to file descriptor $1: only on a
      # terminal, and never with NO_COLOR set or a dumb TERM.
      use_colour() {
        [ -t "$1" ] && [ -z "''${NO_COLOR:-}" ] && [ "''${TERM:-dumb}" != "dumb" ]
      }

      # Help follows goscrape and govendor: the same sections, layout and
      # palette. Unlike theirs, it is only coloured where use_colour allows.
      # printf is a builtin: the app's closure is bash and gawk only, so it
      # mustn't rely on cat or anything else from the caller's PATH.
      usage() {
        local bar="" title="" cmd="" arg="" comment="" flag="" code="" reset=""
        if use_colour "$1"; then
          bar=$'\033[48;2;47;32;129m'
          title=$'\033[1;38;2;255;255;255;48;2;47;32;129m'
          cmd=$'\033[1;38;2;169;128;219m'
          arg=$'\033[38;2;144;108;207m'
          comment=$'\033[38;2;128;219;169m'
          flag=$'\033[1;38;2;219;169;128m'
          code=$'\033[38;2;144;108;207m'
          reset=$'\033[m'
        fi

        heading() {
          if [ -n "$bar" ]; then
            printf '%s   %s%s%s%s%s   %s\n\n' "$bar" "$reset" "$title" "$1" "$reset" "$bar" "$reset"
          else
            printf '%s\n\n' "$1"
          fi
        }
        example() { printf '  %s# %s%s\n  %s\n\n' "$comment" "$1" "$reset" "$2"; }
        option() { printf '  %s%s%s\n          %s\n\n' "$flag" "$1" "$reset" "$2"; }
        entry() { printf '  %s%-*s%s  %s\n' "$code" "$1" "$2" "$reset" "$3"; }

        printf '%s\n' \
          'List the Go toolchains and Go tools available in go-overlay, without' \
          'cloning the repository or compiling anything.' \
          "" \
          'By default, shows the newest release of each minor version. A PREFIX lists' \
          'every release whose version starts with it, so 1.24 lists every 1.24 release' \
          'and 1.2 widens to 1.20 through 1.29. Each row'"'"'s PACKAGE is a flake output' \
          'that can be used directly, for example:' \
          "" \
          '  nix shell github:purpleclay/go-overlay#go_1_25_5' \
          "" \
          'Name a TOOL to list its versions instead. Its PACKAGE is the attribute to' \
          'use from a toolchain, which can also be used directly:' \
          "" \
          "  nix shell 'github:purpleclay/go-overlay#go.tools.gopls.\"0.24.0\"'" \
          ""

        heading USAGE
        printf '  %sgoversions%s %s[FLAGS]%s %s[PREFIX]%s\n' "$cmd" "$reset" "$arg" "$reset" "$arg" "$reset"
        printf '  %sgoversions%s %s[FLAGS]%s %sTOOL%s %s[PREFIX]%s\n' "$cmd" "$reset" "$arg" "$reset" "$arg" "$reset" "$arg" "$reset"
        printf '  %sgoversions%s %s--tools%s %s[--go VERSION]%s\n\n' "$cmd" "$reset" "$flag" "$reset" "$arg" "$reset"

        heading EXAMPLES
        example 'List the newest release of each minor version' "''${cmd}goversions''${reset}"
        example 'List every 1.24 release' "''${cmd}goversions''${reset} 1.24"
        example 'List every release' "''${cmd}goversions''${reset} ''${flag}--all''${reset}"
        example 'List every 1.24 release as a JSON array' \
          "''${cmd}goversions''${reset} ''${flag}--json''${reset} 1.24"
        example 'List every tool and its latest version' "''${cmd}goversions''${reset} ''${flag}--tools''${reset}"
        example 'List the newest gopls release of each minor version' "''${cmd}goversions''${reset} gopls"
        example 'List the gopls versions that work with Go 1.26' \
          "''${cmd}goversions''${reset} gopls ''${flag}--go''${reset} 1.26"

        heading FLAGS
        option '-a, --all' 'list every version, not just the newest per minor'
        option '-g, --go VERSION' 'only list tool versions that work with this Go version'
        option '-h, --help' 'help for goversions'
        option '-j, --json' 'print every matching version as a JSON array'
        option '-t, --tools' 'list every tool and its latest version'
        option '-V, --version' 'print the goversions version'

        heading STATUSES
        entry 10 latest 'the newest release; latest (stable) when it is also the newest'
        entry 10 "" 'stable release'
        entry 10 stable 'the newest stable release, shown when an rc is newer'
        entry 10 supported 'a stable release in a release line Go still supports'
        entry 10 prerelease 'an rc or beta in a release line Go still supports'
        entry 10 eol 'any release in a release line Go no longer supports'
        printf '\n'

        heading 'EXIT CODES'
        entry 1 0 'versions listed'
        entry 1 1 'no versions match PREFIX or --go'
        entry 1 2 'bad flags or arguments'
      }

      prefix=""
      mode="minor"
      json=0
      list_tools=0
      go=""
      positional=()

      while [ "$#" -gt 0 ]; do
        case "$1" in
          -a | --all) mode="all" ;;
          -j | --json) json=1 ;;
          -t | --tools) list_tools=1 ;;
          -g | --go)
            if [ "$#" -lt 2 ] || [ -z "$2" ]; then
              printf 'goversions: --go needs a Go version\n' >&2
              exit 2
            fi
            go="$2"
            shift
            ;;
          -h | --help)
            usage 1
            exit 0
            ;;
          -V | --version)
            printf '%s\n' "${version}"
            exit 0
            ;;
          -*)
            printf 'goversions: unknown option %s\n\n' "$1" >&2
            usage 2 >&2
            exit 2
            ;;
          *) positional+=("$1") ;;
        esac
        shift
      done

      # A first positional argument starting with a digit is a Go prefix;
      # anything else names a tool. No tool name starts with a digit.
      tool=""
      if [ "''${#positional[@]}" -gt 0 ] && [[ ! "''${positional[0]}" =~ ^[0-9] ]]; then
        tool="''${positional[0]}"
        positional=("''${positional[@]:1}")
      fi
      if [ "''${#positional[@]}" -gt 1 ]; then
        printf 'goversions: only one version prefix may be given\n' >&2
        exit 2
      fi
      prefix="''${positional[0]:-}"

      if [ "$list_tools" -eq 1 ] && { [ -n "$tool" ] || [ -n "$prefix" ]; }; then
        printf 'goversions: --tools takes no TOOL or PREFIX\n' >&2
        exit 2
      fi
      if [ -n "$go" ] && [ "$list_tools" -eq 0 ] && [ -z "$tool" ]; then
        printf 'goversions: --go needs a TOOL or --tools\n' >&2
        exit 2
      fi
      if [ -n "$go" ] && [[ ! "$go" =~ ^[0-9]+\.[0-9]+((\.[0-9]+)?|(rc|beta)[0-9]+)$ ]]; then
        printf 'goversions: invalid Go version "%s"\n' "$go" >&2
        exit 2
      fi
      if [ -n "$tool" ] && [[ " $tool_names " != *" $tool "* ]]; then
        printf 'goversions: unknown tool "%s"; valid tools: %s\n' "$tool" "''${tool_names// /, }" >&2
        exit 2
      fi

      color=0
      if use_colour 1; then
        color=1
      fi

      if [ "$list_tools" -eq 1 ] || [ -n "$tool" ]; then
        view="tool"
        if [ "$list_tools" -eq 1 ]; then
          view="summary"
        fi
        filter=0
        if [ -n "$go" ]; then
          filter=1
        fi
        # A prefix is a request to see everything underneath it.
        if [ -n "$prefix" ]; then
          mode="all"
        fi
        GOVERSIONS_TOOL="$tool" GOVERSIONS_PREFIX="$prefix" GOVERSIONS_GO="''${go:-$latest_go}" \
          exec gawk -v view="$view" -v json="$json" -v mode="$mode" -v color="$color" -v filter="$filter" \
          -f "$tools_renderer" "$tools_tsv"
      fi

      # A JSON array of every record, whatever the minor, one per line; consumers
      # filter on newestInMinor. Each line opens {"version":"<v>", so splitting
      # on quotes gives the version as field 4. The prefix is matched as literal
      # text against that value alone, so a quote in it can't match the rest of
      # the line.
      if [ "$json" -eq 1 ]; then
        GOVERSIONS_PREFIX="$prefix" exec gawk '
          split($0, f, "\"") && index(f[4], ENVIRON["GOVERSIONS_PREFIX"]) == 1 {
            printf("%s%s", n++ ? ",\n" : "[\n", $0)
          }
          END {
            if (!n) {
              printf("goversions: no versions match \"%s\"\n", ENVIRON["GOVERSIONS_PREFIX"]) > "/dev/stderr"
              exit 1
            }
            printf("\n]\n")
          }' "$jsonl"
      fi

      # A prefix is a request to see everything underneath it.
      if [ -n "$prefix" ]; then
        mode="all"
      fi

      GOVERSIONS_PREFIX="$prefix" exec gawk -v mode="$mode" -v color="$color" -f "$renderer" "$tsv"
    '';

    # writeShellApplication takes no version argument; set it on the
    # derivation so it reads like goscrape and govendor (e.g. .version).
    derivationArgs = {inherit version;};

    # Exposed so tests can drive the renderer directly with the exact data the
    # wrapper uses.
    passthru = {
      tsv = versionsTsv;
      jsonl = versionsJsonl;
      inherit toolsTsv;
    };

    meta = with lib; {
      homepage = "https://github.com/purpleclay/go-overlay";
      description = "List the Go toolchains and Go tools available in go-overlay";
      mainProgram = pname;
      license = licenses.mit;
      maintainers = with maintainers; [purpleclay];
    };
  }
