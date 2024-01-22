/*
Copyright (c) 2023 - 2024 Purple Clay

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
	"reflect"
	"strings"
	"time"

	"github.com/purpleclay/chomp"
	"github.com/spf13/cobra"
)

// Scrape contains the scraped output from the Go [Download] website, ready
// for serialisation into a Nix attribute set. Any optional [Target] is
// ignored.
//
// [Download]: https://go.dev/dl/
type Scrape struct {
	Date            string  `nix:"d"`
	Version         string  `nix:"v"`
	Darwinx86_64    *Target `nix:"_0,omitempty"`
	DarwinARM64     *Target `nix:"_1,omitempty"`
	Linuxx86        *Target `nix:"_2,omitempty"`
	Linuxx86_64     *Target `nix:"_3,omitempty"`
	LinuxARM64      *Target `nix:"_4,omitempty"`
	LinuxARMv6      *Target `nix:"_5,omitempty"`
	LinuxLoong64    *Target `nix:"_6,omitempty"`
	LinuxMIPS       *Target `nix:"_7,omitempty"`
	LinuxMIPS64     *Target `nix:"_8,omitempty"`
	LinuxMIPS64le   *Target `nix:"_9,omitempty"`
	LinuxMIPSle     *Target `nix:"_10,omitempty"`
	LinuxPPC64      *Target `nix:"_11,omitempty"`
	LinuxPPC64le    *Target `nix:"_12,omitempty"`
	LinuxRiscv64    *Target `nix:"_13,omitempty"`
	LinuxS390x      *Target `nix:"_14,omitempty"`
	AixPPC64        *Target `nix:"_15,omitempty"`
	DragonFlyx86_64 *Target `nix:"_16,omitempty"`
	FreeBSDx86      *Target `nix:"_17,omitempty"`
	FreeBSDx86_64   *Target `nix:"_18,omitempty"`
	FreeBSDARM64    *Target `nix:"_19,omitempty"`
	FreeBSDARMv6    *Target `nix:"_20,omitempty"`
	FreeBSDRiscv64  *Target `nix:"_21,omitempty"`
	Illumosx86_64   *Target `nix:"_22,omitempty"`
	NetBSDx86       *Target `nix:"_23,omitempty"`
	NetBSDx86_64    *Target `nix:"_24,omitempty"`
	NetBSDARM64     *Target `nix:"_25,omitempty"`
	NetBSDARMv6     *Target `nix:"_26,omitempty"`
	OpenBSDx86      *Target `nix:"_27,omitempty"`
	OpenBSDx86_64   *Target `nix:"_28,omitempty"`
	OpenBSDARM64    *Target `nix:"_29,omitempty"`
	OpenBSDARMv6    *Target `nix:"_30,omitempty"`
	Plan9x86        *Target `nix:"_31,omitempty"`
	Plan9x86_64     *Target `nix:"_32,omitempty"`
	Plan9ARMv6      *Target `nix:"_33,omitempty"`
	Solarisx86_64   *Target `nix:"_34,omitempty"`
	Windowsx86      *Target `nix:"_35,omitempty"`
	Windowsx86_64   *Target `nix:"_36,omitempty"`
	WindowsARM64    *Target `nix:"_37,omitempty"`
	WindowsARMv6    *Target `nix:"_38,omitempty"`
}

// Target contains the core details for Nix to download a copy
// of Go for any supported OS-Arch combination
type Target struct {
	SHA string `nix:"s"`
	URL string `nix:"u"`
}

func ScrapeGoDev(rel string) (*Scrape, error) {
	var err error
	if rel == "latest" {
		if rel, err = latestVersion(); err != nil {
			return nil, err
		}
	}

	page, err := get("https://go.dev/dl/")
	if err != nil {
		return nil, err
	}

	return parse(page, rel)
}

func latestVersion() (string, error) {
	ver, err := get("https://go.dev/VERSION?m=text")
	if err != nil {
		return "", err
	}
	_, rel, _ := chomp.Any("go.1234567890")(ver)
	return rel, nil
}

func get(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code returned (%d) when querying %s", resp.StatusCode, url)
	}

	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func parse(in string, rel string) (*Scrape, error) {
	s := &Scrape{
		Date:    time.Now().Format(time.DateOnly),
		Version: rel,
	}

	var ext []string
	var err error

	rem := in
	for {
		// TODO: error should be reported if this hasn't been invoked at least once
		if rem, ext, err = chomp.Pair(href(rel), target())(rem); err != nil {
			break
		}

		if ext[1] != "Archive" {
			continue
		}

		t := &Target{SHA: ext[5], URL: ext[0]}

		switch ext[2] + ext[3] {
		case "macOSx86-64":
			s.Darwinx86_64 = t
		case "macOSARM64":
			s.DarwinARM64 = t
		case "Linuxx86":
			s.Linuxx86 = t
		case "Linuxx86-64":
			s.Linuxx86_64 = t
		case "LinuxARM64":
			s.LinuxARM64 = t
		case "LinuxARMv6":
			s.LinuxARMv6 = t
		case "Linuxloong64":
			s.LinuxLoong64 = t
		case "Linuxmips":
			s.LinuxMIPS = t
		case "Linuxmips64":
			s.LinuxMIPS64 = t
		case "Linuxmips64le":
			s.LinuxMIPS64le = t
		case "Linuxmipsle":
			s.LinuxMIPSle = t
		case "Linuxppc64":
			s.LinuxPPC64 = t
		case "Linuxppc64le":
			s.LinuxPPC64le = t
		case "Linuxriscv64":
			s.LinuxRiscv64 = t
		case "Linuxs390x":
			s.LinuxS390x = t
		case "aixppc64":
			s.AixPPC64 = t
		case "dragonflyx86-64":
			s.DragonFlyx86_64 = t
		case "FreeBSDx86":
			s.FreeBSDx86 = t
		case "FreeBSDx86-64":
			s.FreeBSDx86_64 = t
		case "FreeBSDARM64":
			s.FreeBSDARM64 = t
		case "FreeBSDARMv6":
			s.FreeBSDARMv6 = t
		case "FreeBSDriscv64":
			s.FreeBSDRiscv64 = t
		case "illumosx86-64":
			s.Illumosx86_64 = t
		case "netbsdx86":
			s.NetBSDx86 = t
		case "netbsdx86-64":
			s.NetBSDx86_64 = t
		case "netbsdARM64":
			s.NetBSDARM64 = t
		case "netbsdARMv6":
			s.NetBSDARMv6 = t
		case "openbsdx86":
			s.OpenBSDx86 = t
		case "openbsdx86-64":
			s.OpenBSDx86_64 = t
		case "openbsdARM64":
			s.OpenBSDARM64 = t
		case "openbsdARMv6":
			s.OpenBSDARMv6 = t
		case "plan9x86":
			s.Plan9x86 = t
		case "plan9x86-64":
			s.Plan9x86_64 = t
		case "plan9ARMv6":
			s.Plan9ARMv6 = t
		case "solarisx86-64":
			s.Solarisx86_64 = t
		case "Windowsx86":
			s.Windowsx86 = t
		case "Windowsx86-64":
			s.Windowsx86_64 = t
		case "WindowsARM64":
			s.WindowsARM64 = t
		case "WindowsARMv6":
			s.WindowsARMv6 = t
		}
	}
	return s, nil
}

func href(ver string) chomp.Combinator[string] {
	return func(s string) (string, string, error) {
		rem, ext, err := chomp.All(
			chomp.Until(`<a class="download" href="/dl/`+ver),
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

func (s *Scrape) String() string {
	var buf strings.Builder
	buf.WriteString("{")

	// TODO: tidy this up
	values := reflect.ValueOf(s)
	types := values.Type()
	for i := 0; i < values.NumField(); i++ {
		v := values.Field(i)
		f := types.Field(i)
		tags := strings.Split(f.Tag.Get("nix"), ",")

		var omitempty bool
		for _, tag := range tags[1:] {
			if tag == "omitempty" {
				omitempty = true
			}
		}

		switch f.Type.Kind() {
		case reflect.String:
			buf.WriteString(fmt.Sprintf(`%s="%s";`, tags[0], v.String()))
		case reflect.Ptr:
			if f.Type.Elem().Name() == "Target" {
				isNil := v.Pointer() == 0
				if isNil && omitempty {
					continue
				}

				buf.WriteString(tags[0])
				buf.WriteString("={")

				if !isNil {
					target := reflect.Indirect(v)
					ttypes := target.Type()

					for j := 0; j < target.NumField(); j++ {
						tv := target.Field(j)
						tf := ttypes.Field(j)
						ttags := strings.Split(tf.Tag.Get("nix"), ",")

						switch tf.Type.Kind() {
						case reflect.String:
							buf.WriteString(fmt.Sprintf(`%s="%s";`, ttags[0], tv.String()))
						}
					}
				}

				buf.WriteString("};")
			}
		}
	}

	buf.WriteString("}")
	return buf.String()
}

func DetectVersion(rel string) (string, error) {
	page, err := get("https://go.dev/dl/")
	if err != nil {
		return "", err
	}

	return parseVersion(page, rel)
}

func parseVersion(in, rel string) (string, error) {
	_, ext, err := href(rel)(in)
	if err != nil {
		return "", err
	}

	_, ver, err := chomp.Pair(chomp.Tag("/dl/"), chomp.Any("go.1234567890"))(ext)
	if err != nil {
		return "", err
	}
	return ver[1][:len(ver[1])-1], nil
}

func execute(out io.Writer) error {
	var rel string
	var path string

	cmd := &cobra.Command{
		Use: "go-scrape",
		Short: `Scrapes the Golang website (https://go.dev/dl/) for a specified release and generates a Nix
representation of the output`,
		Example: `
  # Scrape the latest available version of Golang from the website and
  # write to stdout
  $ go-scrape

  # Scrape a specified version of Golang
  $ go-scrape --release go1.20.13

  # Scrape a specified version of Golang and write to a .nix file
  $ go-scrape --release go1.20.13 --output go1-20-13.nix`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := ScrapeGoDev(rel)
			if err != nil {
				return err
			}

			if path != "" {
				return os.WriteFile(path, []byte(s.String()), 0644)
			}

			fmt.Fprintf(out, s.String())
			return nil
		},
	}

	f := cmd.Flags()
	f.StringVarP(&path, "output", "o", "", "the path to a nix file for writing scraped output")
	f.StringVarP(&rel, "release", "r", "latest", "the golang version to scrape from https://go.dev/dl/")

	cmdDetect := &cobra.Command{
		Use: "detect",
		Short: `Scrapes the Golang website (https://go.dev/dl/) to detect the latest version
of a Golang release`,
		Example: `
  # Detect the latest version of Golang from the website
  $ go-scrape detect

  # Detect the latest patch version of a Golang release from the website
  $ go-scrape detect go1.20`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.RangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var ver string
			var err error

			if len(args) == 1 {
				if ver, err = DetectVersion(args[0]); err != nil {
					return err
				}
			} else {
				if ver, err = latestVersion(); err != nil {
					return err
				}
			}

			fmt.Fprintf(out, ver)
			return nil
		},
	}

	cmd.AddCommand(cmdDetect)
	return cmd.Execute()
}

func main() {
	if err := execute(os.Stdout); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}
