package domain

import (
	"slices"
	"strings"
	"testing"
)

// --- ParseDenominatedCopper ---

func TestParseDenominatedCopper(t *testing.T) {
	cases := []struct {
		input string
		want  int
		ok    bool
	}{
		{"2gp", 200, true},
		{"100cp", 100, true},
		{"5sp", 50, true},
		{"3pp", 3000, true},
		{"2ep", 100, true},
		{"1,000gp", 100000, true},
		{"100", 100, true}, // bare = copper
		{"0", 0, true},
		{"10qp", 0, false},  // unknown suffix
		{"abc", 0, false},   // not a number
		{"1.5gp", 0, false}, // non-integer amount
	}
	for _, c := range cases {
		got, err := ParseDenominatedCopper(c.input)
		if c.ok {
			if err != nil {
				t.Errorf("ParseDenominatedCopper(%q) unexpected error: %v", c.input, err)
			} else if got != c.want {
				t.Errorf("ParseDenominatedCopper(%q) = %d, want %d", c.input, got, c.want)
			}
		} else {
			if err == nil {
				t.Errorf("ParseDenominatedCopper(%q) = %d, want error", c.input, got)
			}
		}
	}
}

// --- IsValidCategory ---

func TestIsValidCategory(t *testing.T) {
	valid := []string{"weapon", "armor", "equipment", "tool", "container", "treasure", "mount", "vehicle"}
	for _, v := range valid {
		if !IsValidCategory(v) {
			t.Errorf("IsValidCategory(%q) = false, want true", v)
		}
	}
	invalid := []string{"", "weapons", "WEAPON", "swords", "spell", "misc", "adventuringgear"}
	for _, v := range invalid {
		if IsValidCategory(v) {
			t.Errorf("IsValidCategory(%q) = true, want false", v)
		}
	}
}

// --- TryParseRarity ---

func TestTryParseRarity(t *testing.T) {
	valid := []struct {
		input string
		want  Rarity
	}{
		{"mundane", RarityMundane},
		{"rare", RarityRare},
		{"very-rare", RarityVeryRare},
		{"legendary", RarityLegendary},
		{"common", RarityCommon},
		{"artifact", RarityArtifact},
	}
	for _, v := range valid {
		got, ok := TryParseRarity(v.input)
		if !ok {
			t.Errorf("TryParseRarity(%q) ok=false, want true", v.input)
		} else if got != v.want {
			t.Errorf("TryParseRarity(%q) = %q, want %q", v.input, got, v.want)
		}
	}
	invalid := []string{"", "super-rare", "common1", "mundan"}
	for _, v := range invalid {
		_, ok := TryParseRarity(v)
		if ok {
			t.Errorf("TryParseRarity(%q) ok=true, want false", v)
		}
	}
}

// --- TryParseSourceKind ---

func TestTryParseSourceKind(t *testing.T) {
	cases := []struct {
		input string
		want  SourceKind
	}{
		{"manual", SourceKindManual},
		{"custom", SourceKindCustom},
		{"imported", SourceKindImported},
	}
	for _, v := range cases {
		got, ok := TryParseSourceKind(v.input)
		if !ok {
			t.Errorf("TryParseSourceKind(%q) ok=false, want true", v.input)
		} else if got != v.want {
			t.Errorf("TryParseSourceKind(%q) = %q, want %q", v.input, got, v.want)
		}
	}
	_, ok := TryParseSourceKind("unknown-source")
	if ok {
		t.Error("TryParseSourceKind(\"unknown-source\") ok=true, want false")
	}
}

// --- Strict category and rarity in ParseBulkItemInput ---

func TestParseBulkItemInputUnknownCategory(t *testing.T) {
	preview := ParseBulkItemInput("Sword | swords | mundane | 1 | 100", BulkDefaults{})
	if len(preview.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(preview.Rows))
	}
	if len(preview.Rows[0].Errors) == 0 {
		t.Error("expected error for unknown category, got none")
	}
}

func TestParseBulkItemInputUnknownRarity(t *testing.T) {
	preview := ParseBulkItemInput("Sword | weapon | superrare | 1 | 100", BulkDefaults{})
	if len(preview.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(preview.Rows))
	}
	if len(preview.Rows[0].Errors) == 0 {
		t.Error("expected error for unknown rarity, got none")
	}
}

// --- Denominated copper in bulk input ---

func TestParseBulkItemInputDenominatedValue(t *testing.T) {
	preview := ParseBulkItemInput("Rope | equipment | mundane | 10 | 2gp", BulkDefaults{})
	if len(preview.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(preview.Rows))
	}
	row := preview.Rows[0]
	if len(row.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", row.Errors)
	}
	if row.BaseValueCP != 200 {
		t.Errorf("BaseValueCP = %d, want 200", row.BaseValueCP)
	}
}

func TestParseBulkItemInputDenominatedValueCommas(t *testing.T) {
	preview := ParseBulkItemInput("Fancy Sword | weapon | rare | 3 | 1,000gp", BulkDefaults{})
	if len(preview.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(preview.Rows))
	}
	row := preview.Rows[0]
	if len(row.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", row.Errors)
	}
	if row.BaseValueCP != 100000 {
		t.Errorf("BaseValueCP = %d, want 100000 (1,000gp)", row.BaseValueCP)
	}
}

// --- Description field ---

func TestParseBulkItemInputDescription(t *testing.T) {
	preview := ParseBulkItemInput("Lantern | equipment | mundane | 1 | 50gp | A bright lamp.", BulkDefaults{})
	if len(preview.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(preview.Rows))
	}
	row := preview.Rows[0]
	if len(row.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", row.Errors)
	}
	if row.Description != "A bright lamp." {
		t.Errorf("Description = %q, want \"A bright lamp.\"", row.Description)
	}
}

func TestParseBulkItemInputDescriptionStrayPipe(t *testing.T) {
	// Stray pipe within prose segments should be rejoined with " | ".
	preview := ParseBulkItemInput("Lantern | equipment | mundane | 1 | 50gp | Part one | Part two", BulkDefaults{})
	if len(preview.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(preview.Rows))
	}
	row := preview.Rows[0]
	if len(row.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", row.Errors)
	}
	if row.Description != "Part one | Part two" {
		t.Errorf("Description = %q, want \"Part one | Part two\"", row.Description)
	}
}

// --- Weapon attribute vocabulary ---

func TestParseBulkItemInputWeaponAttributes(t *testing.T) {
	// Full Dagger example from the plan.
	preview := ParseBulkItemInput(
		"Dagger | weapon | mundane | 1 | 2gp | A simple blade. | damage=1d4 | dtype=piercing | props=finesse,light,thrown",
		BulkDefaults{},
	)
	if len(preview.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(preview.Rows))
	}
	row := preview.Rows[0]
	if len(row.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", row.Errors)
	}
	if row.Description != "A simple blade." {
		t.Errorf("Description = %q, want \"A simple blade.\"", row.Description)
	}
	if row.Details.Weapon == nil {
		t.Fatal("WeaponDetails is nil")
	}
	if row.Details.Weapon.DamageDice != "1d4" {
		t.Errorf("DamageDice = %q, want \"1d4\"", row.Details.Weapon.DamageDice)
	}
	if row.Details.Weapon.DamageType != "piercing" {
		t.Errorf("DamageType = %q, want \"piercing\"", row.Details.Weapon.DamageType)
	}
	if len(row.Details.Weapon.Properties) != 3 {
		t.Errorf("Properties len = %d, want 3; got %v", len(row.Details.Weapon.Properties), row.Details.Weapon.Properties)
	}
}

func TestParseBulkItemInputUnknownKeyShapedAttribute(t *testing.T) {
	// Key-shaped but unknown key must produce a row error.
	preview := ParseBulkItemInput("Dagger | weapon | mundane | 1 | 2gp | dmg=1d4", BulkDefaults{})
	if len(preview.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(preview.Rows))
	}
	if len(preview.Rows[0].Errors) == 0 {
		t.Error("expected error for unknown key-shaped attribute, got none")
	}
}

func TestParseBulkItemInputWeaponAttributeOnWrongCategory(t *testing.T) {
	// Weapon-only attribute on a non-weapon category must produce a row error.
	preview := ParseBulkItemInput("Backpack | equipment | mundane | 5 | 200cp | damage=1d4", BulkDefaults{})
	if len(preview.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(preview.Rows))
	}
	if len(preview.Rows[0].Errors) == 0 {
		t.Error("expected error for weapon attribute on non-weapon category, got none")
	}
}

func TestParseBulkItemInputVersatileAttribute(t *testing.T) {
	// versatile= on a weapon row must set VersatileDamageDice.
	preview := ParseBulkItemInput(
		"Quarterstaff | weapon | mundane | 4 | 2sp | A sturdy staff. | damage=1d6 | versatile=1d8 | props=versatile",
		BulkDefaults{},
	)
	if len(preview.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(preview.Rows))
	}
	row := preview.Rows[0]
	if len(row.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", row.Errors)
	}
	if row.Details.Weapon == nil {
		t.Fatal("WeaponDetails is nil")
	}
	if row.Details.Weapon.VersatileDamageDice != "1d8" {
		t.Errorf("VersatileDamageDice = %q, want \"1d8\"", row.Details.Weapon.VersatileDamageDice)
	}
}

func TestParseBulkItemInputVersatileDieImpliesVersatileProperty(t *testing.T) {
	// A versatile die without props=versatile would be incoherent: the item form
	// only reveals the versatile input while that property is checked, so the die
	// would be invisible and silently dropped on the next save. The parser adds
	// the property so the row is self-consistent.
	preview := ParseBulkItemInput("Quarterstaff | weapon | mundane | 4 | 2sp | versatile=1d8", BulkDefaults{})
	row := preview.Rows[0]
	if len(row.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", row.Errors)
	}
	if row.Details.Weapon == nil {
		t.Fatal("WeaponDetails is nil")
	}
	if row.Details.Weapon.VersatileDamageDice != "1d8" {
		t.Errorf("VersatileDamageDice = %q, want \"1d8\"", row.Details.Weapon.VersatileDamageDice)
	}
	if !slices.ContainsFunc(row.Details.Weapon.Properties, func(p string) bool {
		return EqualWeaponProperty(p, "versatile")
	}) {
		t.Errorf("Properties = %v, want it to include \"versatile\"", row.Details.Weapon.Properties)
	}
}

func TestParseBulkItemInputVersatileDieSurvivesLaterPropsAssignment(t *testing.T) {
	// "props=" assigns the properties slice wholesale, so a props= appearing
	// after versatile= must not strip the implied versatile property.
	preview := ParseBulkItemInput("Quarterstaff | weapon | mundane | 4 | 2sp | versatile=1d8 | props=finesse", BulkDefaults{})
	row := preview.Rows[0]
	if len(row.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", row.Errors)
	}
	props := row.Details.Weapon.Properties
	if !slices.ContainsFunc(props, func(p string) bool { return EqualWeaponProperty(p, "versatile") }) {
		t.Errorf("Properties = %v, want it to include \"versatile\"", props)
	}
	if !slices.ContainsFunc(props, func(p string) bool { return EqualWeaponProperty(p, "finesse") }) {
		t.Errorf("Properties = %v, want it to retain \"finesse\"", props)
	}
}

func TestParseBulkItemInputVersatilePropertyNotDuplicated(t *testing.T) {
	// props=versatile alongside versatile=1d8 must not yield the property twice.
	preview := ParseBulkItemInput("Quarterstaff | weapon | mundane | 4 | 2sp | versatile=1d8 | props=versatile", BulkDefaults{})
	row := preview.Rows[0]
	count := 0
	for _, p := range row.Details.Weapon.Properties {
		if EqualWeaponProperty(p, "versatile") {
			count++
		}
	}
	if count != 1 {
		t.Errorf("versatile property appears %d times in %v, want exactly 1", count, row.Details.Weapon.Properties)
	}
}

func TestParseBulkItemInputVersatileAttributeOnWrongCategory(t *testing.T) {
	// versatile= on a non-weapon category must produce a row error.
	preview := ParseBulkItemInput("Cloak | equipment | mundane | 1 | 50gp | versatile=1d8", BulkDefaults{})
	if len(preview.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(preview.Rows))
	}
	if len(preview.Rows[0].Errors) == 0 {
		t.Error("expected error for versatile attribute on non-weapon category, got none")
	}
}

// --- Per-row boolean overrides (pointer bools) ---

func TestParseBulkItemInputPerRowBoolOverrides(t *testing.T) {
	preview := ParseBulkItemInput("Rope | equipment | mundane | 10 | 100 | stackable=true | magical=false", BulkDefaults{})
	if len(preview.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(preview.Rows))
	}
	row := preview.Rows[0]
	if len(row.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", row.Errors)
	}
	if row.IsStackable == nil {
		t.Fatal("IsStackable is nil, want *true")
	}
	if *row.IsStackable != true {
		t.Errorf("IsStackable = %v, want true", *row.IsStackable)
	}
	if row.IsMagical == nil {
		t.Fatal("IsMagical is nil, want *false")
	}
	if *row.IsMagical != false {
		t.Errorf("IsMagical = %v, want false", *row.IsMagical)
	}
}

// --- Location capture (domain layer only: names, no ID resolution) ---

func TestParseBulkItemInputVaultAndParentCapture(t *testing.T) {
	preview := ParseBulkItemInput("Rope | equipment | mundane | 10 | 100 | vault=Lyra | parent=Backpack", BulkDefaults{})
	if len(preview.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(preview.Rows))
	}
	row := preview.Rows[0]
	if len(row.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", row.Errors)
	}
	if row.VaultName != "Lyra" {
		t.Errorf("VaultName = %q, want \"Lyra\"", row.VaultName)
	}
	if row.ContainerName != "Backpack" {
		t.Errorf("ContainerName = %q, want \"Backpack\"", row.ContainerName)
	}
	// Domain layer never resolves names to IDs.
	if row.ResolvedVaultID != "" {
		t.Errorf("ResolvedVaultID = %q, want empty", row.ResolvedVaultID)
	}
	if row.ResolvedContainerID != "" {
		t.Errorf("ResolvedContainerID = %q, want empty", row.ResolvedContainerID)
	}
}

// --- BulkFormatSpec vocabulary coverage ---

func TestBulkFormatSpecCoversVocabulary(t *testing.T) {
	spec := BulkFormatSpec()
	for _, cat := range Categories() {
		if !strings.Contains(spec, cat) {
			t.Errorf("BulkFormatSpec missing category %q", cat)
		}
	}
	for _, r := range Rarities() {
		if !strings.Contains(spec, r) {
			t.Errorf("BulkFormatSpec missing rarity %q", r)
		}
	}
	for _, key := range BulkAttributeKeys() {
		if !strings.Contains(spec, key) {
			t.Errorf("BulkFormatSpec missing attribute key %q", key)
		}
	}
}
