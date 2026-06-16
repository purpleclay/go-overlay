package modproxy

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const mitLicenseText = `MIT License

Copyright (c) 2024 Example Author

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
`

func TestDetectLicenseMIT(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "LICENSE"), []byte(mitLicenseText), 0o644))

	id, err := detectLicense(dir)
	require.NoError(t, err)
	assert.Equal(t, "MIT", id)
}

func TestDetectLicenseIgnoresSupplementaryFiles(t *testing.T) {
	// LICENSE.google and LICENSE-THIRD-PARTY are supplementary attribution
	// files present in some tool repos (e.g. gofumpt, staticcheck). They
	// should not be scanned — only LICENSE is checked.
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "LICENSE.google"), []byte(mitLicenseText), 0o644))

	id, err := detectLicense(dir)
	require.NoError(t, err)
	assert.Empty(t, id)
}

func TestDetectLicenseMissing(t *testing.T) {
	dir := t.TempDir()

	id, err := detectLicense(dir)
	require.NoError(t, err)
	assert.Empty(t, id)
}

func TestDetectLicenseDeprecatedSPDXNormalized(t *testing.T) {
	// licensecheck returns "GPL-3.0" (deprecated) for a plain "Version 3" GPL
	// text with no "or later" grant; detectLicense must normalise it to the
	// canonical "GPL-3.0-only".
	dir := t.TempDir()
	content, err := os.ReadFile(filepath.Join("testdata", "GPL-3.0.txt"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "LICENSE"), content, 0o644))

	id, err := detectLicense(dir)
	require.NoError(t, err)
	assert.Equal(t, "GPL-3.0-only", id)
}

func TestDetectLicenseUnrecognised(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "LICENSE"), []byte("All rights reserved.\n"), 0o644))

	id, err := detectLicense(dir)
	require.NoError(t, err)
	assert.Empty(t, id)
}
