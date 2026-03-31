package suggest_test

import (
	"testing"

	"github.com/purpleclay/go-overlay/examples/cobra-cli/internal/suggest"
)

func TestCountVowels(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"Bryn", 0},
		{"Bob", 1},
		{"Mike", 2},
		{"Alice", 3},
		{"Eloise", 4},
		{"ALICE", 3},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := suggest.CountVowels(tt.input); got != tt.expected {
				t.Errorf("CountVowels(%q) = %d, want %d", tt.input, got, tt.expected)
			}
		})
	}
}

func TestForName(t *testing.T) {
	tests := []struct {
		input         string
		expectedBreed string
	}{
		{"Bryn", "Shiba Inu"},
		{"Bob", "Dachshund"},
		{"Mike", "Border Collie"},
		{"Alice", "Golden Retriever"},
		{"Eloise", "Pug"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := suggest.ForName(tt.input).Name; got != tt.expectedBreed {
				t.Errorf("ForName(%q).Name = %q, want %q", tt.input, got, tt.expectedBreed)
			}
		})
	}
}
