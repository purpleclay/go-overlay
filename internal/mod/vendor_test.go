package mod

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func manifestTOML(schema int, version, hash string) []byte {
	var v string
	if version != "" {
		v = fmt.Sprintf("version = %q\n", version)
	}
	return fmt.Appendf(nil, "schema = %d\n%shash = %q\n", schema, v, hash)
}

func TestCheckDriftForSchemaMismatch(t *testing.T) {
	vendor := NewVendor(WithVendoredVersion("v0.10.0"))
	data := manifestTOML(schemaVersion-1, "v0.10.0", "sha256-abc")

	result, drifted := vendor.checkDrift("go.mod", data, "sha256-abc")

	require.True(t, drifted)
	assert.Equal(t, statusDrift, result.status)
	assert.Contains(t, result.message, fmt.Sprintf("schema v%d", schemaVersion-1))
	assert.Contains(t, result.message, fmt.Sprintf("schema v%d", schemaVersion))
}

func TestCheckDriftForSchemaMismatchSuppressesHashCheck(t *testing.T) {
	vendor := NewVendor(WithVendoredVersion("v0.10.0"))
	data := manifestTOML(schemaVersion-1, "v0.10.0", "sha256-old")

	result, drifted := vendor.checkDrift("go.mod", data, "sha256-new")

	require.True(t, drifted)
	assert.Equal(t, statusDrift, result.status)
	assert.NotContains(t, result.message, "hash")
}

func TestCheckDriftForHashMismatch(t *testing.T) {
	vendor := NewVendor()
	data := manifestTOML(schemaVersion, "", "sha256-old")

	result, drifted := vendor.checkDrift("go.mod", data, "sha256-new")

	require.True(t, drifted)
	assert.Equal(t, statusDrift, result.status)
	assert.Contains(t, result.message, "hash: go.mod has changed")
	assert.Contains(t, result.message, "sha256-new")
	assert.Contains(t, result.message, "sha256-old")
}

func TestCheckDriftForMinorVersionMismatchWarning(t *testing.T) {
	vendor := NewVendor(WithVendoredVersion("v0.10.0"))
	data := manifestTOML(schemaVersion, "v0.9.0", "sha256-abc")

	result, drifted := vendor.checkDrift("go.mod", data, "sha256-abc")

	require.True(t, drifted)
	assert.Equal(t, statusWarning, result.status)
	assert.Contains(t, result.message, "govendor version mismatch: v0.9.0 → v0.10.0")
	assert.Contains(t, result.message, "--check --strict to enforce")
}

func TestCheckDriftForMinorVersionMismatchStrictPromotesToDrift(t *testing.T) {
	vendor := NewVendor(WithVendoredVersion("v0.10.0"), WithStrict())
	data := manifestTOML(schemaVersion, "v0.9.0", "sha256-abc")

	result, drifted := vendor.checkDrift("go.mod", data, "sha256-abc")

	require.True(t, drifted)
	assert.Equal(t, statusDrift, result.status)
	assert.Contains(t, result.message, "govendor version mismatch: v0.9.0 → v0.10.0")
}

func TestCheckDriftForMajorVersionMismatch(t *testing.T) {
	vendor := NewVendor(WithVendoredVersion("v2.0.0"))
	data := manifestTOML(schemaVersion, "v1.0.0", "sha256-abc")

	result, drifted := vendor.checkDrift("go.mod", data, "sha256-abc")

	require.True(t, drifted)
	assert.Equal(t, statusDrift, result.status)
	assert.Contains(t, result.message, "govendor version mismatch: v1.0.0 → v2.0.0")
	assert.Contains(t, result.message, "incompatible major version")
}

func TestCheckDriftForMajorVersionMismatchAndHashMismatch(t *testing.T) {
	vendor := NewVendor(WithVendoredVersion("v2.0.0"))
	data := manifestTOML(schemaVersion, "v1.0.0", "sha256-old")

	result, drifted := vendor.checkDrift("go.mod", data, "sha256-new")

	require.True(t, drifted)
	assert.Equal(t, statusDrift, result.status)
	assert.Contains(t, result.message, "govendor version mismatch: v1.0.0 → v2.0.0")
	assert.Contains(t, result.message, "hash: go.mod has changed")
}
