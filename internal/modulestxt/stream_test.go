package modulestxt_test

import (
	"testing"

	"github.com/purpleclay/go-overlay/internal/modulestxt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStreamLineRejectsAnnotationBeforeHeader(t *testing.T) {
	var stream modulestxt.Stream

	err := stream.Line("## explicit; go 1.25.0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "annotation before module header")
}
