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
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	fd, _ := os.ReadFile("testdata/index-20240121.html")

	s, err := parse(string(fd), "go1.21.6")
	require.NoError(t, err)
	assert.Equal(t, "go1.21.6", s.Version)
	assert.Equal(t, "31d6ecca09010ab351e51343a5af81d678902061fee871f912bdd5ef4d778850", s.Darwinx86_64.SHA)
	assert.Equal(t, "/dl/go1.21.6.darwin-amd64.tar.gz", s.Darwinx86_64.URL)
	assert.Equal(t, "a8f55bdee2bb285c2d9d3da8d8e18682224b21fe15f439798add9b33a0040968", s.AixPPC64.SHA)
	assert.Equal(t, "/dl/go1.21.6.aix-ppc64.tar.gz", s.AixPPC64.URL)
	assert.Equal(t, "fafb3ba1d415876fa08d37370cac6aaef4263b119da99906b8f147bcfb0a74fd", s.OpenBSDx86.SHA)
	assert.Equal(t, "/dl/go1.21.6.openbsd-386.tar.gz", s.OpenBSDx86.URL)
	assert.Equal(t, "b2b187a44da8842a1dd159282e3dbe4e0c03891ce7a213d358a70a7be9587589", s.WindowsARMv6.SHA)
	assert.Equal(t, "/dl/go1.21.6.windows-arm.zip", s.WindowsARMv6.URL)
}

func TestParseVersion(t *testing.T) {
	fd, _ := os.ReadFile("testdata/index-20240121.html")

	ver, err := parseVersion(string(fd), "go1.20")
	require.NoError(t, err)
	assert.Equal(t, "go1.20.13", ver)
}
