package main

import (
	"fmt"
	"strings"

	"github.com/purpleclay/chomp"
)

func goVersion() chomp.Combinator[string] {
	return func(s string) (string, string, error) {
		rem, rel, err := chomp.All(chomp.Tag("go"), chomp.Any(".1234567890rc"))(s)
		if err != nil {
			return rem, "", err
		}

		return rem, strings.TrimSuffix(rel[1], "."), nil
	}
}

func href(ver string) chomp.Combinator[string] {
	normalizedVersion := ver
	if !strings.HasPrefix(normalizedVersion, "go") {
		normalizedVersion = "go" + normalizedVersion
	}

	return func(s string) (string, string, error) {
		rem, ext, err := chomp.All(
			chomp.Until(fmt.Sprintf(`<a class="download" href="/dl/%s.`, normalizedVersion)),
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

func target() chomp.Combinator[[]string] {
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
