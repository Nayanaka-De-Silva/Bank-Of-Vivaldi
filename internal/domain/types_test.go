package domain

import "testing"

func TestBreakdownCP(t *testing.T) {
	tests := []struct {
		input int
		want  CoinBreakdown
	}{
		{0, CoinBreakdown{PP: 0, GP: 0, EP: 0, SP: 0, CP: 0}},
		{1357, CoinBreakdown{PP: 1, GP: 3, EP: 1, SP: 0, CP: 7}},
		{100, CoinBreakdown{PP: 0, GP: 1, EP: 0, SP: 0, CP: 0}},
		{250, CoinBreakdown{PP: 0, GP: 2, EP: 1, SP: 0, CP: 0}},
		{7, CoinBreakdown{PP: 0, GP: 0, EP: 0, SP: 0, CP: 7}},
		{1000, CoinBreakdown{PP: 1, GP: 0, EP: 0, SP: 0, CP: 0}},
	}

	for _, tc := range tests {
		got := BreakdownCP(tc.input)
		if got != tc.want {
			t.Errorf("BreakdownCP(%d) = %+v, want %+v", tc.input, got, tc.want)
		}
	}
}

func TestBreakdownCPRoundTrip(t *testing.T) {
	cases := []int{0, 1, 7, 10, 50, 99, 100, 250, 999, 1000, 1357, 9999, 12345}
	for _, n := range cases {
		b := BreakdownCP(n)
		got := (b.PP * 1000) + (b.GP * 100) + (b.EP * 50) + (b.SP * 10) + b.CP
		if got != n {
			t.Errorf("BreakdownCP(%d) round-trip = %d, want %d (breakdown: %+v)", n, got, n, b)
		}
	}
}

func TestHumanizeLabel(t *testing.T) {
	tests := map[string]string{
		"":                     "",
		"equipment":            "Equipment",
		"very-rare":            "Very Rare",
		"wondrous-item":        "Wondrous Item",
		"pull_request":         "Pull Request",
		"  adventuring-gear  ": "Adventuring Gear",
	}

	for input, want := range tests {
		if got := HumanizeLabel(input); got != want {
			t.Fatalf("HumanizeLabel(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestWeaponCategories(t *testing.T) {
	want := []string{"simple-melee", "simple-ranged", "martial-melee", "martial-ranged"}
	got := WeaponCategories()

	if len(got) != len(want) {
		t.Fatalf("WeaponCategories() returned %d categories, want %d", len(got), len(want))
	}
	for i, category := range want {
		if got[i] != category {
			t.Fatalf("WeaponCategories()[%d] = %q, want %q", i, got[i], category)
		}
	}
}

func TestIsValidWeaponCategory(t *testing.T) {
	tests := map[string]bool{
		"simple-melee":   true,
		"simple-ranged":  true,
		"martial-melee":  true,
		"martial-ranged": true,
		"":               false,
		"martial":        false,
		"ranged":         false,
		"Simple Melee":   false,
	}

	for input, want := range tests {
		if got := IsValidWeaponCategory(input); got != want {
			t.Fatalf("IsValidWeaponCategory(%q) = %v, want %v", input, got, want)
		}
	}
}

func TestIsRangedWeaponCategory(t *testing.T) {
	tests := map[string]bool{
		"simple-ranged":  true,
		"martial-ranged": true,
		"simple-melee":   false,
		"martial-melee":  false,
		"":               false,
	}

	for input, want := range tests {
		if got := IsRangedWeaponCategory(input); got != want {
			t.Fatalf("IsRangedWeaponCategory(%q) = %v, want %v", input, got, want)
		}
	}
}
