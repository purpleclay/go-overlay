package scrape

import (
	"fmt"
	"strings"

	"github.com/purpleclay/chomp"
)

// prereleaseTag matches an optional "rc" or "beta" pre-release suffix
// followed by its number (e.g. "rc1", "beta2"). Matched as a literal token
// rather than folding its letters into a character class, since a class
// containing 'a' would also match the start of "aix" - a real target OS
// name that immediately follows the version in scraped download links
// (e.g. "go1.21.4.aix-ppc64.tar.gz").
var prereleaseTag = chomp.Opt(chomp.Recognize(chomp.Pair(
	chomp.First(chomp.Tag("rc"), chomp.Tag("beta")),
	chomp.Any("1234567890"),
)))

func GoVersion() chomp.Combinator[string] {
	return func(s string) (string, string, error) {
		rem, rel, err := chomp.All(chomp.Tag("go"), chomp.Any(".1234567890"), prereleaseTag)(s)
		if err != nil {
			return rem, "", err
		}

		return rem, strings.TrimSuffix(rel[1]+rel[2], "."), nil
	}
}

func SeekDownloadSection(version string) chomp.Combinator[string] {
	normalizedVersion := version
	if !strings.HasPrefix(normalizedVersion, "go") {
		normalizedVersion = "go" + normalizedVersion
	}

	return func(s string) (string, string, error) {
		rem, _, err := chomp.Until(fmt.Sprintf(`id="%s"`, normalizedVersion))(s)
		if err != nil {
			return rem, "", err
		}
		return rem, "", nil
	}
}

func Href(version string) chomp.Combinator[string] {
	hrefPrefix := `<a class="download" href="/dl/go`
	if version != "" {
		normalizedVersion := version
		if !strings.HasPrefix(normalizedVersion, "go") {
			normalizedVersion = "go" + normalizedVersion
		}

		// An exact rc/beta version (e.g. "go1.19beta1") or a full patch
		// version (e.g. "go1.21.4") needs a trailing "." to avoid matching a
		// longer version that shares the same prefix (go1.19beta1 would
		// otherwise also match go1.19beta10, and go1.21.4 would match
		// go1.21.40). A bare minor prefix (e.g. "go1.19") must not get one,
		// since listVersions relies on matching every version under it.
		isExactPrerelease := strings.Contains(normalizedVersion, "rc") || strings.Contains(normalizedVersion, "beta")
		hrefVersion := normalizedVersion
		if isExactPrerelease || strings.Count(normalizedVersion, ".") >= 2 {
			hrefVersion = normalizedVersion + "."
		}

		hrefPrefix = fmt.Sprintf(`<a class="download" href="/dl/%s`, hrefVersion)
	}

	return func(s string) (string, string, error) {
		rem, ext, err := chomp.All(
			chomp.Until(hrefPrefix),
			chomp.Delimited(chomp.Tag(`<a class="download" href="`), chomp.Until(`"`), chomp.Tag(`"`)),
			eol())(s)
		if err != nil {
			return rem, "", err
		}

		return rem, ext[1], nil
	}
}

func eol() chomp.Combinator[string] {
	return func(s string) (string, string, error) {
		rem, _, err := chomp.Pair(chomp.Until("\n"), chomp.Crlf())(s)
		if err != nil {
			return rem, "", err
		}

		return rem, "", nil
	}
}

func Target() chomp.Combinator[[]string] {
	return func(s string) (string, []string, error) {
		return chomp.All(
			chomp.Repeat(tableCell("<td>", "</td>"), 4),
			chomp.S(tableCell("<td><tt>", "</tt></td>")),
		)(s)
	}
}

func tableCell(deliml, delimr string) chomp.Combinator[string] {
	return func(s string) (string, string, error) {
		rem, ext, err := chomp.Pair(
			chomp.Any(" "),
			chomp.Delimited(chomp.Tag(deliml), chomp.Until(delimr), chomp.Tag(delimr)))(s)
		if err != nil {
			return rem, "", err
		}

		rem, _, err = eol()(rem)
		if err != nil {
			return rem, "", err
		}

		return rem, ext[1], nil
	}
}
