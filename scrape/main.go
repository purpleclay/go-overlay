/*
Copyright (c) 2023 Purple Clay

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/purpleclay/chomp"
	"github.com/spf13/cobra"
)

// TODO: write custom marshaller that understands the nix attribute

type Scrape struct {
	Date        string `nix:"d"`
	Version     string `nix:"v"`
	DarwinAmd64 Target `nix:"_0,omitempty"`
	DarwinArm64 Target `nix:"_1,omitempty"`
	LinuxAmd32  Target ``
	LinuxAmd64  Target ``
	LinuxArm64  Target ``
	LinuxArmV6  Target ``
	// Other Linux
	AixPPC64       Target ``
	DragonFlyAmd64 Target ``
	FreeBSDAmd32   Target ``
	FreeBSDAmd64   Target ``
	FreeBSDArm64   Target ``
	FreeBSDArmV6   Target ``
	FreeBSDRiscV64 Target ``
	IllumosAmd64   Target ``
	// NetBSD
	// OpenBSD
	// Plan9
	// Solaris
	// Windows
}

// TODO: do all new versions of Go include these os + arch combinations?

/*
- aixppc64
- macOSx86-64
- macOSARM64
- dragonflyx86-64
- FreeBSDx86
- FreeBSDx86-64
- FreeBSDARM64
- FreeBSDARMv6
- FreeBSDriscv64
- illumosx86-64
- Linuxx86
- Linuxx86-64
- LinuxARM64
- LinuxARMv6
Linuxloong64
Linuxmips
Linuxmips64
Linuxmips64le
Linuxmipsle
Linuxppc64
Linuxppc64le
Linuxriscv64
Linuxs390x
netbsdx86
netbsdx86-64
netbsdARM64
netbsdARMv6
openbsdx86
openbsdx86-64
openbsdARM64
openbsdARMv6
plan9x86
plan9x86-64
plan9ARMv6
solarisx86-64
Windowsx86
Windowsx86-64
WindowsARM64
WindowsARMv6
*/

type Target struct {
	SHA string `nix:"s"`
	URL string `nix:"u"`
}

func execute(out io.Writer) error {
	var rel string

	cmd := &cobra.Command{
		Use:           "go-scrape",
		Short:         "Scrapes the Golang Website (https://go.dev/dl/) for a specified release and generates a Nix representation of the output",
		Example:       "TODO",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: optionally write output to a file or to stdout
			return scrape(rel)
		},
	}

	f := cmd.Flags()
	f.StringVarP(&rel, "release", "r", "latest", "the golang version to scrape from https://go.dev/dl/")

	return cmd.Execute()
}

func scrape(rel string) error {
	// TODO: if latest, query :> https://go.dev/VERSION?m=text (returns the latest version)

	resp, err := http.Get("https://go.dev/dl/")
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		// TODO: report error
	}

	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// TODO: scrape the latest version (HTTP Get and parse the output)
	// TODO: serialise the output as a Nix attribute set
	s := parse(string(data), rel)
	fmt.Printf("%#v\n", s)
	return nil
}

func parse(in string, rel string) Scrape {
	s := Scrape{
		Date:    time.Now().Format(time.DateOnly),
		Version: rel,
	}

	var ext []string
	var err error

	rem := in
	for {
		if rem, ext, err = chomp.Pair(href(rel), target())(rem); err != nil {
			// TODO: handle error properly
			break
		}

		if ext[1] != "Archive" {
			continue
		}

		t := Target{URL: ext[0], SHA: ext[5]}

		switch ext[2] + ext[3] {
		case "macOSx86-64":
			s.DarwinAmd64 = t
		case "macOSARM64":
			s.DarwinArm64 = t
		}
	}
	return s
}

// TODO: combine href and target together
func href(ver string) chomp.Combinator[string] {
	return func(s string) (string, string, error) {
		rem, ext, err := chomp.All(
			chomp.Until(`href="/dl/`+ver),
			chomp.Delimited(chomp.Tag(`href="`), chomp.Until(`"`), chomp.Tag(`"`)),
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

func main() {
	if err := execute(os.Stdout); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}
