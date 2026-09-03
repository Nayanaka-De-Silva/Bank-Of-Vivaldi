package application

import (
	"testing"

	"bank-of-vivaldi/internal/domain"
)

// makeVaultSummary builds a minimal VaultSummary for filter tests.
func makeVaultSummary(name string, kind domain.VaultKind, carryHundredths, maxHundredths, itemCount int, state domain.EncumbranceState) VaultSummary {
	return VaultSummary{
		Vault: domain.Vault{
			ID:            name,
			CharacterName: name,
			Kind:          kind,
		},
		TotalCarryWeightHundredths: carryHundredths,
		MaxCarryWeightHundredths:   maxHundredths,
		ItemCount:                  itemCount,
		EncumbranceState:           state,
	}
}

func TestVaultListFiltersEmptyMatchesAll(t *testing.T) {
	t.Parallel()

	summaries := []VaultSummary{
		makeVaultSummary("Alice", domain.VaultKindPC, 500, 1500, 3, domain.EncumbranceStateNormal),
		makeVaultSummary("Bob", domain.VaultKindNPC, 1000, 1500, 7, domain.EncumbranceStateEncumbered),
	}

	got, err := VaultListFilters{}.Apply(summaries)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 results, got %d", len(got))
	}
}

func TestVaultListFiltersKind(t *testing.T) {
	t.Parallel()

	summaries := []VaultSummary{
		makeVaultSummary("Alice", domain.VaultKindPC, 0, 1500, 0, domain.EncumbranceStateNormal),
		makeVaultSummary("Bob", domain.VaultKindNPC, 0, 1500, 0, domain.EncumbranceStateNormal),
	}

	got, err := VaultListFilters{Kind: "npc"}.Apply(summaries)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Vault.CharacterName != "Bob" {
		t.Fatalf("expected only Bob (NPC), got %v", got)
	}
}

func TestVaultListFiltersQuery(t *testing.T) {
	t.Parallel()

	summaries := []VaultSummary{
		makeVaultSummary("Aragorn", domain.VaultKindPC, 0, 1500, 0, domain.EncumbranceStateNormal),
		makeVaultSummary("Gimli", domain.VaultKindPC, 0, 1500, 0, domain.EncumbranceStateNormal),
	}

	got, err := VaultListFilters{Query: "ara"}.Apply(summaries)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Vault.CharacterName != "Aragorn" {
		t.Fatalf("expected only Aragorn, got %v", got)
	}
}

func TestVaultListFiltersCarryBounds(t *testing.T) {
	t.Parallel()

	// carry: Alice=5.00 lb (500 hundredths), Bob=10.00 lb (1000), Charlie=20.00 lb (2000)
	summaries := []VaultSummary{
		makeVaultSummary("Alice", domain.VaultKindPC, 500, 3000, 0, domain.EncumbranceStateNormal),
		makeVaultSummary("Bob", domain.VaultKindPC, 1000, 3000, 0, domain.EncumbranceStateNormal),
		makeVaultSummary("Charlie", domain.VaultKindPC, 2000, 3000, 0, domain.EncumbranceStateNormal),
	}

	// MinCarryLB=6, MaxCarryLB=15 → only Bob (10 lb)
	got, err := VaultListFilters{MinCarryLB: "6", MaxCarryLB: "15"}.Apply(summaries)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Vault.CharacterName != "Bob" {
		t.Fatalf("expected only Bob, got %v", got)
	}
}

func TestVaultListFiltersItemCountBounds(t *testing.T) {
	t.Parallel()

	summaries := []VaultSummary{
		makeVaultSummary("Alice", domain.VaultKindPC, 0, 1500, 2, domain.EncumbranceStateNormal),
		makeVaultSummary("Bob", domain.VaultKindPC, 0, 1500, 5, domain.EncumbranceStateNormal),
		makeVaultSummary("Charlie", domain.VaultKindPC, 0, 1500, 10, domain.EncumbranceStateNormal),
	}

	got, err := VaultListFilters{MinItems: "3", MaxItems: "8"}.Apply(summaries)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Vault.CharacterName != "Bob" {
		t.Fatalf("expected only Bob, got %v", got)
	}
}

func TestVaultListFiltersEncumbranceState(t *testing.T) {
	t.Parallel()

	summaries := []VaultSummary{
		makeVaultSummary("Alice", domain.VaultKindPC, 0, 1500, 0, domain.EncumbranceStateNormal),
		makeVaultSummary("Bob", domain.VaultKindPC, 0, 1500, 0, domain.EncumbranceStateEncumbered),
		makeVaultSummary("Charlie", domain.VaultKindPC, 0, 1500, 0, domain.EncumbranceStateHeavilyEncumbered),
	}

	got, err := VaultListFilters{State: "encumbered"}.Apply(summaries)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Vault.CharacterName != "Bob" {
		t.Fatalf("expected only Bob (encumbered), got %v", got)
	}
}

func TestVaultListFiltersSortByName(t *testing.T) {
	t.Parallel()

	summaries := []VaultSummary{
		makeVaultSummary("Zara", domain.VaultKindPC, 0, 1500, 0, domain.EncumbranceStateNormal),
		makeVaultSummary("Alice", domain.VaultKindPC, 0, 1500, 0, domain.EncumbranceStateNormal),
	}

	got, err := VaultListFilters{SortBy: "name"}.Apply(summaries)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got[0].Vault.CharacterName != "Alice" || got[1].Vault.CharacterName != "Zara" {
		t.Fatalf("expected [Alice Zara], got [%s %s]", got[0].Vault.CharacterName, got[1].Vault.CharacterName)
	}
}

func TestVaultListFiltersSortByCarry(t *testing.T) {
	t.Parallel()

	summaries := []VaultSummary{
		makeVaultSummary("Heavy", domain.VaultKindPC, 2000, 3000, 0, domain.EncumbranceStateNormal),
		makeVaultSummary("Light", domain.VaultKindPC, 500, 3000, 0, domain.EncumbranceStateNormal),
	}

	got, err := VaultListFilters{SortBy: "carry"}.Apply(summaries)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got[0].Vault.CharacterName != "Light" {
		t.Fatalf("expected Light first when sorting by carry ascending, got %s", got[0].Vault.CharacterName)
	}
}

func TestVaultListFiltersActiveFilterCount(t *testing.T) {
	t.Parallel()

	empty := VaultListFilters{}
	if empty.ActiveFilterCount() != 0 {
		t.Fatalf("expected 0 active filters for empty, got %d", empty.ActiveFilterCount())
	}
	if empty.HasActiveFilters() {
		t.Fatal("expected HasActiveFilters to be false for empty filters")
	}

	withKind := VaultListFilters{Kind: "pc"}
	if withKind.ActiveFilterCount() != 1 {
		t.Fatalf("expected 1 active filter, got %d", withKind.ActiveFilterCount())
	}
	if !withKind.HasActiveFilters() {
		t.Fatal("expected HasActiveFilters to be true")
	}

	multi := VaultListFilters{Kind: "npc", Query: "bob", MinCarryLB: "1"}
	if multi.ActiveFilterCount() != 3 {
		t.Fatalf("expected 3 active filters, got %d", multi.ActiveFilterCount())
	}
}

func TestVaultListFiltersInvalidBoundsReturnError(t *testing.T) {
	t.Parallel()

	_, err := VaultListFilters{MinCarryLB: "notanumber"}.Apply(nil)
	if err == nil {
		t.Fatal("expected error for non-numeric min carry bound")
	}
}
