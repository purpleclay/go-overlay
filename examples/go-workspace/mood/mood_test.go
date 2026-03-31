package mood_test

import (
	"testing"

	"github.com/purpleclay/go-overlay/examples/go-workspace/mood"
)

func TestFor(t *testing.T) {
	cases := []struct {
		score int
		label string
	}{
		{1, "Volcanic"},
		{2, "Fuming"},
		{3, "Grumpy"},
		{4, "Irritable"},
		{5, "Uneasy"},
		{6, "Meh"},
		{7, "Okay"},
		{8, "Decent"},
		{9, "Bright"},
		{10, "Brilliant"},
		{11, "Transcendent"},
	}

	for _, c := range cases {
		got := mood.For(c.score)
		if got.Label != c.label {
			t.Errorf("For(%d).Label = %q, want %q", c.score, got.Label, c.label)
		}
		if len(got.Anecdote) == 0 {
			t.Errorf("For(%d).Anecdote is empty", c.score)
		}
		if len(got.Color) == 0 {
			t.Errorf("For(%d).Color is empty", c.score)
		}
	}
}

func TestForOutOfRange(t *testing.T) {
	for _, score := range []int{0, -1, 12, 100} {
		got := mood.For(score)
		if got.Label != "" || got.Anecdote != "" {
			t.Errorf("For(%d) = %+v, want empty Feeling", score, got)
		}
	}
}
