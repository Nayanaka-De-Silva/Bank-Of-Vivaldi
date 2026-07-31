package domain

import (
	"testing"
)

func TestWeaponPropertiesReturnsTenInPHBOrder(t *testing.T) {
	t.Parallel()

	props := WeaponProperties()
	if len(props) != 10 {
		t.Fatalf("expected 10 weapon properties, got %d", len(props))
	}

	wantKeys := []string{
		"ammunition", "finesse", "heavy", "light", "loading",
		"reach", "special", "thrown", "two-handed", "versatile",
	}
	for i, want := range wantKeys {
		if props[i].Key != want {
			t.Errorf("props[%d].Key = %q, want %q", i, props[i].Key, want)
		}
		if props[i].Label == "" {
			t.Errorf("props[%d].Label is empty", i)
		}
		if props[i].Description == "" {
			t.Errorf("props[%d].Description is empty", i)
		}
	}
}

func TestLookupWeaponPropertyNormalizesInput(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input   string
		wantKey string
		wantOK  bool
	}{
		{"Two-Handed", "two-handed", true},
		{"two handed", "two-handed", true},
		{" THROWN ", "thrown", true},
		{"ammunition", "ammunition", true},
		{"", "", false},
		{"glowing", "", false},
	}

	for _, tc := range cases {
		prop, ok := LookupWeaponProperty(tc.input)
		if ok != tc.wantOK {
			t.Errorf("LookupWeaponProperty(%q) ok = %v, want %v", tc.input, ok, tc.wantOK)
			continue
		}
		if ok && prop.Key != tc.wantKey {
			t.Errorf("LookupWeaponProperty(%q).Key = %q, want %q", tc.input, prop.Key, tc.wantKey)
		}
	}
}

func TestCanonicalWeaponPropertyPreservesUnknownValues(t *testing.T) {
	t.Parallel()

	// Known values return the canonical key.
	if got := CanonicalWeaponProperty("Two-Handed"); got != "two-handed" {
		t.Errorf("CanonicalWeaponProperty(%q) = %q, want %q", "Two-Handed", got, "two-handed")
	}
	if got := CanonicalWeaponProperty("two handed"); got != "two-handed" {
		t.Errorf("CanonicalWeaponProperty(%q) = %q, want %q", "two handed", got, "two-handed")
	}

	// Unknown values come back trimmed and unchanged.
	if got := CanonicalWeaponProperty("glowing"); got != "glowing" {
		t.Errorf("CanonicalWeaponProperty(%q) = %q, want %q", "glowing", got, "glowing")
	}
	if got := CanonicalWeaponProperty("  silvered  "); got != "silvered" {
		t.Errorf("CanonicalWeaponProperty(%q) = %q, want %q", "  silvered  ", got, "silvered")
	}

	// Empty input returns empty string.
	if got := CanonicalWeaponProperty(""); got != "" {
		t.Errorf("CanonicalWeaponProperty(%q) = %q, want %q", "", got, "")
	}
}

func TestNormalizeWeaponPropertiesCanonicalizesDedupesDropsBlankPreservesOrder(t *testing.T) {
	t.Parallel()

	// Canonicalizes known properties from variant spellings.
	got := NormalizeWeaponProperties([]string{"Two Handed", "THROWN", "ammunition"})
	want := []string{"two-handed", "thrown", "ammunition"}
	if !slicesEqual(got, want) {
		t.Errorf("NormalizeWeaponProperties(%v) = %v, want %v", []string{"Two Handed", "THROWN", "ammunition"}, got, want)
	}

	// Deduplicates: second "two-handed" (via "two handed") is dropped.
	got = NormalizeWeaponProperties([]string{"two-handed", "two handed"})
	want = []string{"two-handed"}
	if !slicesEqual(got, want) {
		t.Errorf("dedup case: got %v, want %v", got, want)
	}

	// Drops blanks.
	got = NormalizeWeaponProperties([]string{"", "heavy", ""})
	want = []string{"heavy"}
	if !slicesEqual(got, want) {
		t.Errorf("blank-drop case: got %v, want %v", got, want)
	}

	// Unknown values are kept but still deduplicated by their trimmed form.
	got = NormalizeWeaponProperties([]string{"silvered", "silvered", "heavy"})
	want = []string{"silvered", "heavy"}
	if !slicesEqual(got, want) {
		t.Errorf("unknown-dedup case: got %v, want %v", got, want)
	}

	// Nil input returns nil (no panic).
	if got := NormalizeWeaponProperties(nil); got != nil {
		t.Errorf("nil input: got %v, want nil", got)
	}
}

func TestResolveWeaponPropertiesGivesUnknownValuesEmptyDescription(t *testing.T) {
	t.Parallel()

	props := ResolveWeaponProperties([]string{"thrown", "silvered"})
	if len(props) != 2 {
		t.Fatalf("expected 2 resolved properties, got %d", len(props))
	}

	// Known property must have a non-empty description.
	if props[0].Key != "thrown" {
		t.Errorf("props[0].Key = %q, want %q", props[0].Key, "thrown")
	}
	if props[0].Description == "" {
		t.Errorf("known property %q must have a non-empty description", props[0].Key)
	}

	// Unknown property must have an empty description.
	if props[1].Key != "silvered" {
		t.Errorf("props[1].Key = %q, want %q", props[1].Key, "silvered")
	}
	if props[1].Description != "" {
		t.Errorf("unknown property %q must have an empty description, got %q", props[1].Key, props[1].Description)
	}
	if props[1].Label == "" {
		t.Errorf("unknown property %q must still have a humanized label", props[1].Key)
	}
}

func TestWeaponPropertyChoicesAppendsUnknownStoredValuesAfterCanonical(t *testing.T) {
	t.Parallel()

	choices := WeaponPropertyChoices([]string{"ammunition", "silvered"})

	// The ten canonical entries come first.
	if len(choices) != 11 {
		t.Fatalf("expected 11 choices (10 canonical + 1 legacy), got %d", len(choices))
	}

	// All first ten must be Known=true.
	canonical := WeaponProperties()
	for i, prop := range canonical {
		if choices[i].Known != true {
			t.Errorf("choices[%d].Known = false, want true (canonical entry)", i)
		}
		if choices[i].Key != prop.Key {
			t.Errorf("choices[%d].Key = %q, want %q", i, choices[i].Key, prop.Key)
		}
	}

	// "ammunition" must be Selected=true.
	if !choices[0].Selected {
		t.Errorf("expected ammunition (choices[0]) to be Selected=true")
	}

	// Other canonical entries must be Selected=false.
	for i := 1; i < 10; i++ {
		if choices[i].Selected {
			t.Errorf("choices[%d] (%s) should be Selected=false", i, choices[i].Key)
		}
	}

	// The 11th entry is the legacy "silvered".
	legacy := choices[10]
	if legacy.Known {
		t.Errorf("legacy entry Known = true, want false")
	}
	if !legacy.Selected {
		t.Errorf("legacy entry Selected = false, want true")
	}
	if legacy.Key != "silvered" {
		t.Errorf("legacy entry Key = %q, want %q", legacy.Key, "silvered")
	}
	if legacy.Description != "" {
		t.Errorf("legacy entry Description must be empty, got %q", legacy.Description)
	}

	// A canonical variant in selected ("Two Handed" normalizes to "two-handed") must
	// mark the canonical entry selected but NOT add a legacy entry.
	choices2 := WeaponPropertyChoices([]string{"Two Handed"})
	if len(choices2) != 10 {
		t.Fatalf("expected 10 choices when selected is a canonical variant, got %d", len(choices2))
	}
	// Find "two-handed" and confirm it's selected.
	found := false
	for _, c := range choices2 {
		if c.Key == "two-handed" {
			if !c.Selected {
				t.Errorf("two-handed must be Selected=true when selected=%q", "Two Handed")
			}
			found = true
		}
	}
	if !found {
		t.Errorf("expected to find two-handed in choices")
	}

	// Nil selected returns exactly 10 entries, none selected.
	choices3 := WeaponPropertyChoices(nil)
	if len(choices3) != 10 {
		t.Fatalf("WeaponPropertyChoices(nil): expected 10 choices, got %d", len(choices3))
	}
	for _, c := range choices3 {
		if c.Selected {
			t.Errorf("WeaponPropertyChoices(nil): expected no Selected entries, but %q is selected", c.Key)
		}
	}
}

func TestWeaponPropertyOptionsMergesCanonicalAndLegacyFacets(t *testing.T) {
	t.Parallel()

	// No stored values: returns all 10 canonical keys.
	opts := WeaponPropertyOptions(nil)
	if len(opts) != 10 {
		t.Fatalf("WeaponPropertyOptions(nil): expected 10 options, got %d: %v", len(opts), opts)
	}
	if opts[0] != "ammunition" || opts[9] != "versatile" {
		t.Errorf("expected canonical order starting with ammunition and ending with versatile, got %v", opts)
	}

	// A non-canonical stored value is appended.
	opts = WeaponPropertyOptions([]string{"silvered"})
	if len(opts) != 11 {
		t.Fatalf("expected 11 options with legacy %q, got %d: %v", "silvered", len(opts), opts)
	}
	if opts[10] != "silvered" {
		t.Errorf("expected legacy %q at end, got %q", "silvered", opts[10])
	}

	// A canonical value in stored is NOT duplicated.
	opts = WeaponPropertyOptions([]string{"ammunition"})
	if len(opts) != 10 {
		t.Fatalf("canonical stored value should not be duplicated: got %d options", len(opts))
	}

	// A canonical variant ("Two Handed") in stored is resolved to canonical and not duplicated.
	opts = WeaponPropertyOptions([]string{"Two Handed"})
	if len(opts) != 10 {
		t.Fatalf("canonical variant %q should not be duplicated: got %d options", "Two Handed", len(opts))
	}
}

func TestEqualWeaponPropertyIsCaseAndSeparatorTolerant(t *testing.T) {
	t.Parallel()

	cases := []struct {
		a, b string
		want bool
	}{
		{"two-handed", "two-handed", true},
		{"two-handed", "Two Handed", true},
		{"Two-Handed", "two handed", true},
		{"thrown", "THROWN", true},
		{"thrown", "finesse", false},
		{"", "thrown", false},
		{"thrown", "", false},
		{"glowing", "glowing", true}, // unknown values match themselves
		{"glowing", "GLOWING", true}, // Slugify lowercases, so case variants match even without lookup
		{"glowing", "shiny", false},  // different unknown values do not match
	}

	for _, tc := range cases {
		if got := EqualWeaponProperty(tc.a, tc.b); got != tc.want {
			t.Errorf("EqualWeaponProperty(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

// slicesEqual reports whether two string slices are equal.
func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
