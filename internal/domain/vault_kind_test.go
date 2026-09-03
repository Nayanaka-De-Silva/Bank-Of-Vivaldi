package domain

import "testing"

func TestParseVaultKind(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input   string
		wantOK  bool
		wantVal VaultKind
	}{
		{"pc", true, VaultKindPC},
		{"npc", true, VaultKindNPC},
		{"PC", true, VaultKindPC},
		{"NPC", true, VaultKindNPC},
		{"", false, ""},
		{"player", false, ""},
		{"monster", false, ""},
	}
	for _, tc := range cases {
		got, ok := ParseVaultKind(tc.input)
		if ok != tc.wantOK || got != tc.wantVal {
			t.Errorf("ParseVaultKind(%q) = (%q, %v), want (%q, %v)", tc.input, got, ok, tc.wantVal, tc.wantOK)
		}
	}
}

func TestVaultKindValid(t *testing.T) {
	t.Parallel()

	if !VaultKindPC.Valid() {
		t.Error("VaultKindPC should be valid")
	}
	if !VaultKindNPC.Valid() {
		t.Error("VaultKindNPC should be valid")
	}
	if VaultKind("").Valid() {
		t.Error("empty VaultKind should be invalid")
	}
	if VaultKind("monster").Valid() {
		t.Error("unknown VaultKind should be invalid")
	}
}

func TestVaultKindLabel(t *testing.T) {
	t.Parallel()

	if got := VaultKindPC.Label(); got != "PC" {
		t.Errorf("VaultKindPC.Label() = %q, want %q", got, "PC")
	}
	if got := VaultKindNPC.Label(); got != "NPC" {
		t.Errorf("VaultKindNPC.Label() = %q, want %q", got, "NPC")
	}
}

func TestVaultKinds(t *testing.T) {
	t.Parallel()

	kinds := VaultKinds()
	if len(kinds) != 2 {
		t.Fatalf("VaultKinds() returned %d kinds, want 2", len(kinds))
	}
	if kinds[0] != VaultKindPC || kinds[1] != VaultKindNPC {
		t.Errorf("VaultKinds() = %v, want [pc npc]", kinds)
	}
}
