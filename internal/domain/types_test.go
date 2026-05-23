package domain

import "testing"

func TestHumanizeLabel(t *testing.T) {
	tests := map[string]string{
		"":                "",
		"equipment":       "Equipment",
		"very-rare":       "Very Rare",
		"wondrous-item":   "Wondrous Item",
		"pull_request":    "Pull Request",
		"  adventuring-gear  ": "Adventuring Gear",
	}

	for input, want := range tests {
		if got := HumanizeLabel(input); got != want {
			t.Fatalf("HumanizeLabel(%q) = %q, want %q", input, got, want)
		}
	}
}
