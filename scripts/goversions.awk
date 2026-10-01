# Renders the goversions table from the TSV goversions.nix writes into
# the store. POSIX awk apart from /dev/stderr, so output is identical under
# gawk and mawk.
#
# Input columns:
#   1 version  2 released  3 minor  4 status  5 package  6 newest-in-minor
#
# Supplied by the wrapper:
#   mode    -v: "minor" (newest release per minor) or "all"
#   color   -v: 1 to emit ANSI escapes
#   GOVERSIONS_PREFIX   environment: version prefix filter, "" for none.
#     Read from ENVIRON rather than -v because -v processes backslash escapes
#     (and gawk warns about them), which would alter user input.

BEGIN {
    FS = "\t"
    prefix = ENVIRON["GOVERSIONS_PREFIX"]

    # Seed column widths from the headers so a narrow result set still lines up.
    wv = length("VERSION")
    wd = length("RELEASED")
    ws = length("STATUS")

    if (color) {
        bold = "\033[1m"
        dim = "\033[2m"
        green = "\033[32m"
        cyan = "\033[36m"
        yellow = "\033[33m"
        reset = "\033[0m"
    }
}

{
    if (prefix != "" && index($1, prefix) != 1)
        next

    if (mode == "minor" && $6 != "1") {
        collapsed++
        next
    }

    n++
    version[n] = $1
    released[n] = $2
    minor[n] = $3
    status[n] = $4
    pkg[n] = $5

    if (length($1) > wv) wv = length($1)
    if (length($2) > wd) wd = length($2)
    if (length($4) > ws) ws = length($4)
}

function paint(s, c) {
    return c == "" ? s : c s reset
}

function statusColor(s) {
    if (s == "latest" || s == "latest (stable)") return green
    if (s == "stable") return cyan
    if (s == "prerelease") return yellow
    if (s == "eol") return dim
    return ""
}

function row(v, d, s, p) {
    return sprintf("%-*s  %-*s  %-*s  %s", wv, v, wd, d, ws, s, p)
}

END {
    if (n == 0) {
        printf("goversions: no versions match \"%s\"\n", prefix) > "/dev/stderr"
        exit 1
    }

    printf("%s\n", paint(row("VERSION", "RELEASED", "STATUS", "PACKAGE"), bold))
    for (i = 1; i <= n; i++)
        printf("%s\n", paint(row(version[i], released[i], status[i], pkg[i]), statusColor(status[i])))

    if (collapsed > 0)
        printf("\n%s\n", paint(sprintf("%d more release%s hidden - pass a prefix (e.g. %s) or --all", \
            collapsed, collapsed == 1 ? "" : "s", minor[1]), dim))
}
