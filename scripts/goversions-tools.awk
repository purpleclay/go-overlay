# Renders goversions' tool views from the TSV goversions.nix writes into the
# store. POSIX awk apart from /dev/stderr, so output is identical under gawk
# and mawk.
#
# Input columns, one row per tool version, each tool's rows newest first:
#   1 tool  2 version  3 released  4 minor  5 prerelease (0/1)  6 min go
#   7 license  8-12 min go parsed by lib/version.nix: major, minor, patch,
#   stage, counter
#
# Supplied by the wrapper:
#   view    -v: "tool" (every version of one tool) or "summary" (every tool)
#   json    -v: 1 to print a JSON array instead of a table
#   mode    -v: "minor" (newest stable release per minor) or "all"
#   color   -v: 1 to emit ANSI escapes
#   filter  -v: 1 to drop versions that need a newer Go than GOVERSIONS_GO
#   GOVERSIONS_TOOL     environment: the tool for view=tool
#   GOVERSIONS_PREFIX   environment: version prefix filter, "" for none
#   GOVERSIONS_GO       environment: the Go version "latest" is resolved
#     against, already validated by the wrapper. Read from ENVIRON rather
#     than -v because -v processes backslash escapes.

BEGIN {
    FS = "\t"
    tool = ENVIRON["GOVERSIONS_TOOL"]
    prefix = ENVIRON["GOVERSIONS_PREFIX"]
    goVersion = ENVIRON["GOVERSIONS_GO"]
    parseGo(goVersion)

    if (color) {
        bold = "\033[1m"
        dim = "\033[2m"
        green = "\033[32m"
        yellow = "\033[33m"
        reset = "\033[0m"
    }
}

# The same rules as lib/version.nix's parseVersion: major.minor[.patch], or
# major.minorrcN / major.minorbetaN. beta is stage 0, rc 1, stable 2.
function parseGo(v,    p, n) {
    n = split(v, p, ".")
    x[1] = p[1] + 0
    x[3] = 0
    if (match(p[2], /rc|beta/)) {
        x[2] = substr(p[2], 1, RSTART - 1) + 0
        x[4] = substr(p[2], RSTART, RLENGTH) == "rc" ? 1 : 0
        x[5] = substr(p[2], RSTART + RLENGTH) + 0
    } else {
        x[2] = p[2] + 0
        x[3] = n > 2 ? p[3] + 0 : 0
        x[4] = 2
        x[5] = 0
    }
}

# Whether row i's minimum Go is no newer than GOVERSIONS_GO, matching
# mk-tool-set.nix's compareVersions goVersion required >= 0.
function compatible(i,    k) {
    for (k = 1; k <= 5; k++) {
        if (key[i, k] < x[k]) return 1
        if (key[i, k] > x[k]) return 0
    }
    return 1
}

view == "summary" || $1 == tool {
    n++
    name[n] = $1
    version[n] = $2
    released[n] = $3
    minor[n] = $4
    pre[n] = $5
    minGo[n] = $6
    license[n] = $7
    for (k = 1; k <= 5; k++)
        key[n, k] = $(7 + k) + 0
}

function paint(s, c) {
    return c == "" ? s : c s reset
}

function pkg(i) {
    return "go.tools." name[i] ".\"" version[i] "\""
}

function q(s) {
    gsub(/"/, "\\\"", s)
    return "\"" s "\""
}

function emit(line) {
    printf("%s%s", count++ ? ",\n" : "[\n", line)
}

function closeJson() {
    printf("%s]\n", count ? "\n" : "[\n")
}

END {
    if (view == "summary")
        summary()
    else
        versions()
}

# One row per tool: the newest version that works with GOVERSIONS_GO, as
# go.tools.<name>.latest resolves it, or none.
function summary(    i, t, cols, w, rows, best, r) {
    w[1] = length("TOOL"); w[2] = length("LATEST"); w[3] = length("RELEASED"); w[4] = length("MIN GO")
    for (i = 1; i <= n; i++) {
        if (name[i] != t) {
            t = name[i]
            rows++
            tools[rows] = t
            best[rows] = 0
        }
        if (!best[rows] && compatible(i))
            best[rows] = i
    }

    if (json) {
        for (r = 1; r <= rows; r++) {
            i = best[r]
            if (i)
                emit(sprintf("{\"tool\":%s,\"version\":%s,\"date\":%s,\"go\":%s,\"license\":%s,\"package\":%s}", \
                    q(tools[r]), q(version[i]), q(released[i]), q(minGo[i]), q(license[i]), q(pkg(i))))
            else
                emit(sprintf("{\"tool\":%s,\"version\":null,\"date\":null,\"go\":null,\"license\":null,\"package\":null}", q(tools[r])))
        }
        closeJson()
        return
    }

    for (r = 1; r <= rows; r++) {
        i = best[r]
        if (length(tools[r]) > w[1]) w[1] = length(tools[r])
        if (i && length(version[i]) > w[2]) w[2] = length(version[i])
        if (i && length(released[i]) > w[3]) w[3] = length(released[i])
        if (i && length(minGo[i]) > w[4]) w[4] = length(minGo[i])
    }

    printf("%s\n", paint(sprintf("%-*s  %-*s  %-*s  %-*s  %s", w[1], "TOOL", w[2], "LATEST", w[3], "RELEASED", w[4], "MIN GO", "LICENSE"), bold))
    for (r = 1; r <= rows; r++) {
        i = best[r]
        if (i)
            printf("%-*s  %-*s  %-*s  %-*s  %s\n", w[1], tools[r], w[2], version[i], w[3], released[i], w[4], minGo[i], license[i])
        else
            printf("%s\n", paint(sprintf("%-*s  %-*s  %-*s  %-*s  %s", w[1], tools[r], w[2], "none", w[3], "-", w[4], "-", "-"), dim))
    }
}

# Every version of one tool, newest first. "latest" is the newest version
# that works with GOVERSIONS_GO; by default only that and the newest stable
# release of each minor are shown. A table marks latest and pre-releases by
# colour only, as the version string already says which is a pre-release;
# JSON carries status so scripts needn't rely on record order.
function versions(    i, latest, shown, wv, wd, wg, status, hidden, seen, first) {
    for (i = 1; i <= n; i++) {
        if (compatible(i)) {
            latest = i
            break
        }
    }

    wv = length("VERSION"); wd = length("RELEASED"); wg = length("MIN GO")
    for (i = 1; i <= n; i++) {
        if (filter && !compatible(i)) continue
        if (prefix != "" && index(version[i], prefix) != 1) continue

        status[i] = i == latest ? "latest" : (pre[i] == "1" ? "prerelease" : "")

        if (!json && mode == "minor") {
            if (i != latest && (pre[i] == "1" || seen[minor[i]])) {
                # Suggested as a prefix: its minor always has a hidden release
                if (!hidden++)
                    hiddenMinor = minor[i]
                continue
            }
            if (pre[i] != "1") seen[minor[i]] = 1
        }

        shown++
        row[shown] = i
        if (length(version[i]) > wv) wv = length(version[i])
        if (length(released[i]) > wd) wd = length(released[i])
        if (length(minGo[i]) > wg) wg = length(minGo[i])
    }

    if (!shown) {
        if (prefix != "" && filter)
            printf("goversions: no %s versions match \"%s\" with Go %s\n", tool, prefix, goVersion) > "/dev/stderr"
        else if (prefix != "")
            printf("goversions: no %s versions match \"%s\"\n", tool, prefix) > "/dev/stderr"
        else
            printf("goversions: no %s versions work with Go %s\n", tool, goVersion) > "/dev/stderr"
        exit 1
    }

    if (json) {
        for (s = 1; s <= shown; s++) {
            i = row[s]
            emit(sprintf("{\"tool\":%s,\"version\":%s,\"date\":%s,\"go\":%s,\"license\":%s,\"status\":%s,\"package\":%s}", \
                q(name[i]), q(version[i]), q(released[i]), q(minGo[i]), q(license[i]), q(status[i]), q(pkg(i))))
        }
        closeJson()
        return
    }

    printf("%s\n", paint(sprintf("%-*s  %-*s  %-*s  %s", wv, "VERSION", wd, "RELEASED", wg, "MIN GO", "PACKAGE"), bold))
    for (s = 1; s <= shown; s++) {
        i = row[s]
        line = sprintf("%-*s  %-*s  %-*s  %s", wv, version[i], wd, released[i], wg, minGo[i], pkg(i))
        printf("%s\n", paint(line, status[i] == "latest" ? green : (status[i] == "prerelease" ? yellow : "")))
    }

    if (hidden > 0)
        printf("\n%s\n", paint(sprintf("%d more release%s hidden - pass a prefix (e.g. %s) or --all", \
            hidden, hidden == 1 ? "" : "s", hiddenMinor), dim))
}
