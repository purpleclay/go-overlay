# Builds the `goversions` CLI from a list of versionInfo records.
#
# Everything is resolved at evaluation time and written into the store as a
# TSV that scripts/goversions.awk renders. The only runtime input is the
# user's version prefix, so running it needs bash and gawk and compiles
# nothing. records is a parameter so tests can build the app against a fixed
# fixture rather than the real, ever-growing version list.
{
  lib,
  writeText,
  writeShellApplication,
  gawk,
  records,
}: let
  pname = "goversions";
  version = "v0.2.0";

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
          'List the Go toolchains available in go-overlay, without cloning the' \
          'repository or compiling anything.' \
          "" \
          'By default, shows the newest release of each minor version. A PREFIX lists' \
          'every release whose version starts with it, so 1.24 lists every 1.24 release' \
          'and 1.2 widens to 1.20 through 1.29. Each row'"'"'s PACKAGE is a flake output' \
          'that can be used directly, for example:' \
          "" \
          '  nix shell github:purpleclay/go-overlay#go_1_25_5' \
          ""

        heading USAGE
        printf '  %sgoversions%s %s[FLAGS]%s %s[PREFIX]%s\n\n' "$cmd" "$reset" "$arg" "$reset" "$arg" "$reset"

        heading EXAMPLES
        example 'List the newest release of each minor version' "''${cmd}goversions''${reset}"
        example 'List every 1.24 release' "''${cmd}goversions''${reset} 1.24"
        example 'List every release' "''${cmd}goversions''${reset} ''${flag}--all''${reset}"
        example 'List every 1.24 release as a JSON array' \
          "''${cmd}goversions''${reset} ''${flag}--json''${reset} 1.24"

        heading FLAGS
        option '-a, --all' 'list every version, not just the newest per minor'
        option '-h, --help' 'help for goversions'
        option '-j, --json' 'print every matching version as a JSON array'
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
        entry 1 1 'no versions match PREFIX'
        entry 1 2 'bad flags or arguments'
      }

      prefix=""
      mode="minor"
      json=0

      while [ "$#" -gt 0 ]; do
        case "$1" in
          -a | --all) mode="all" ;;
          -j | --json) json=1 ;;
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
          *)
            if [ -n "$prefix" ]; then
              printf 'goversions: only one version prefix may be given\n' >&2
              exit 2
            fi
            prefix="$1"
            ;;
        esac
        shift
      done

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

      color=0
      if use_colour 1; then
        color=1
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
    };

    meta = with lib; {
      homepage = "https://github.com/purpleclay/go-overlay";
      description = "List the Go toolchains available in go-overlay";
      mainProgram = pname;
      license = licenses.mit;
      maintainers = with maintainers; [purpleclay];
    };
  }
