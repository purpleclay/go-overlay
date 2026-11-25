package godev

import (
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gotest.tools/v3/golden"
)

func TestScrape(t *testing.T) {
	fd, err := os.ReadFile("testdata/index-20260215.html")
	require.NoError(t, err)

	// Use a fixed date for reproducible tests
	fixedDate := time.Date(2025, 12, 8, 0, 0, 0, 0, time.UTC)

	s, err := parse(string(fd), "go1.21.6", fixedDate)
	require.NoError(t, err)

	manifest := s.String()
	golden.Assert(t, manifest, "go1.21.6.nix.golden")
}

func scrapeFor(ver, date string) *Scrape {
	return &Scrape{
		Version: ver,
		Date:    date,
		Targets: []Target{{
			System: "x86_64-linux",
			SHA256: "fake",
			URL:    "https://go.dev/dl/go" + ver + ".linux-amd64.tar.gz",
		}},
	}
}

func TestWriteManifestsWritesEachManifestAndTheIndex(t *testing.T) {
	dir := t.TempDir()
	writeFakeGoManifests(t, dir, map[string]string{"1.27.1": "2026-09-01"})

	err := writeManifests([]*Scrape{scrapeFor("1.27.2", "2026-10-07")}, dir, io.Discard)
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(dir, "1.27.2.nix"))
	index, err := os.ReadFile(filepath.Join(dir, "index.nix"))
	require.NoError(t, err)
	assert.Contains(t, string(index), `"1.27.2" = {date = "2026-10-07";};`)
	assert.Contains(t, string(index), `"1.27.1" = {date = "2026-09-01";};`)
}

func TestWriteManifestsCreatesAMissingOutputDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "manifests", "go")

	err := writeManifests([]*Scrape{scrapeFor("1.27.2", "2026-10-07")}, dir, io.Discard)
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(dir, "1.27.2.nix"))
	assert.FileExists(t, filepath.Join(dir, "index.nix"))
}

func TestWriteManifestsRepairsAMalformedManifestForARegeneratedVersion(t *testing.T) {
	dir := t.TempDir()
	writeFakeGoManifests(t, dir, map[string]string{"1.27.1": "2026-09-01"})
	malformed := "{\n  version = \"1.27.2\";\n}\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "1.27.2.nix"), []byte(malformed), 0o644))

	err := writeManifests([]*Scrape{scrapeFor("1.27.2", "2026-10-07")}, dir, io.Discard)
	require.NoError(t, err)

	repaired, err := os.ReadFile(filepath.Join(dir, "1.27.2.nix"))
	require.NoError(t, err)
	assert.Contains(t, string(repaired), `date = "2026-10-07";`)
	index, err := os.ReadFile(filepath.Join(dir, "index.nix"))
	require.NoError(t, err)
	assert.Contains(t, string(index), `"1.27.2" = {date = "2026-10-07";};`)
}

func TestWriteManifestsRejectsAMalformedManifestItIsNotRegenerating(t *testing.T) {
	dir := t.TempDir()
	malformed := "{\n  version = \"1.27.1\";\n}\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "1.27.1.nix"), []byte(malformed), 0o644))

	err := writeManifests([]*Scrape{scrapeFor("1.27.2", "2026-10-07")}, dir, io.Discard)
	assert.ErrorContains(t, err, "1.27.1.nix has no top-level date")

	assert.NoFileExists(t, filepath.Join(dir, "1.27.2.nix"))
}

func TestWriteManifestsRejectsASymlinkedManifestWithoutFollowingIt(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(t.TempDir(), "outside.txt")
	require.NoError(t, os.WriteFile(target, []byte("unrelated\n"), 0o644))
	require.NoError(t, os.Symlink(target, filepath.Join(dir, "1.27.2.nix")))

	err := writeManifests([]*Scrape{scrapeFor("1.27.2", "2026-10-07")}, dir, io.Discard)
	assert.ErrorContains(t, err, "1.27.2.nix is not a regular file")

	outside, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "unrelated\n", string(outside), "the symlink target outside the output directory must not be written")
	assert.NoFileExists(t, filepath.Join(dir, "index.nix"))
}

func TestWriteManifestsRejectsASymlinkedIndexWithoutFollowingIt(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(t.TempDir(), "outside.txt")
	require.NoError(t, os.WriteFile(target, []byte("unrelated\n"), 0o644))
	require.NoError(t, os.Symlink(target, filepath.Join(dir, "index.nix")))

	err := writeManifests([]*Scrape{scrapeFor("1.27.2", "2026-10-07")}, dir, io.Discard)
	assert.ErrorContains(t, err, "index.nix is not a regular file")

	outside, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "unrelated\n", string(outside), "the symlink target outside the output directory must not be written")
	assert.NoFileExists(t, filepath.Join(dir, "1.27.2.nix"))
}

func TestWriteManifestsRejectsAnUnrelatedNixFileBeforeWritingAnything(t *testing.T) {
	dir := t.TempDir()
	writeFakeGoManifests(t, dir, map[string]string{"1.27.1": "2026-09-01"})
	require.NoError(t, os.WriteFile(filepath.Join(dir, "default.nix"), []byte("{}\n"), 0o644))

	err := writeManifests([]*Scrape{scrapeFor("1.27.2", "2026-10-07")}, dir, io.Discard)
	assert.ErrorContains(t, err, "default.nix is not named after a Go version")

	assert.NoFileExists(t, filepath.Join(dir, "1.27.2.nix"))
	assert.NoFileExists(t, filepath.Join(dir, "index.nix"))
}
