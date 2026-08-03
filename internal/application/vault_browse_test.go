package application

import (
	"context"
	"testing"

	"bank-of-vivaldi/internal/domain"
)

// vaultBrowseFixture mirrors compendiumFixture's shape: vault-1 owns a root
// item plus a container with a nested item, vault-2 owns one item of its own
// (to prove cross-vault isolation), and a compendium-owned item proves vaults
// never leak compendium items into their browse results.
func vaultBrowseFixture() *Service {
	store := newFakeStore()
	store.vaults["vault-1"] = domain.Vault{ID: "vault-1", CharacterName: "Aragorn", StrengthScore: 16}
	store.vaults["vault-2"] = domain.Vault{ID: "vault-2", CharacterName: "Legolas", StrengthScore: 14}

	pack := domain.Item{
		ID:                 "pack",
		Name:               "Traveler's Pack",
		Category:           "container",
		Rarity:             domain.RarityMundane,
		WeightHundredthsLB: 500,
		BaseValueCP:        200,
		Quantity:           1,
		IsContainer:        true,
		Location:           domain.ItemLocation{Kind: domain.LocationKindVaultRoot, OwnerVaultID: "vault-1"},
	}
	pack.Details.Container = &domain.ContainerDetails{MaxWeightHundredthsLB: 10000}

	rations := domain.Item{
		ID:                 "rations",
		Name:               "Rations",
		Category:           "adventuring-gear",
		Rarity:             domain.RarityMundane,
		WeightHundredthsLB: 200,
		BaseValueCP:        500,
		Quantity:           5,
		// Nested inside vault-1's pack; OwnerVaultID mirrors what
		// Service.resolveLocation stamps onto persisted container children.
		Location: domain.ItemLocation{Kind: domain.LocationKindContainer, ParentContainerItemID: "pack", OwnerVaultID: "vault-1"},
	}

	sword := domain.Item{
		ID:                 "sword",
		Name:               "Aragorn's Sword",
		Category:           "weapon",
		Rarity:             domain.RarityRare,
		WeightHundredthsLB: 300,
		BaseValueCP:        150000,
		Quantity:           1,
		Location:           domain.ItemLocation{Kind: domain.LocationKindVaultRoot, OwnerVaultID: "vault-1"},
	}
	sword.Details.Weapon = &domain.WeaponDetails{WeaponClass: "martial-melee", DamageType: "slashing"}

	elvenBow := domain.Item{
		ID:                 "elven-bow",
		Name:               "Elven Bow",
		Category:           "weapon",
		Rarity:             domain.RarityUncommon,
		WeightHundredthsLB: 200,
		BaseValueCP:        75000,
		Quantity:           1,
		Location:           domain.ItemLocation{Kind: domain.LocationKindVaultRoot, OwnerVaultID: "vault-2"},
	}

	compendiumTorch := domain.Item{
		ID:                 "torch",
		Name:               "Torch",
		Category:           "equipment",
		Rarity:             domain.RarityMundane,
		WeightHundredthsLB: 100,
		BaseValueCP:        10,
		Quantity:           1,
		Location:           domain.ItemLocation{Kind: domain.LocationKindCompendiumRoot},
	}

	for _, item := range []domain.Item{pack, rations, sword, elvenBow, compendiumTorch} {
		store.items[item.ID] = item
	}

	return NewService(store)
}

func TestBrowseVaultScopesToTheVaultsItems(t *testing.T) {
	t.Parallel()

	service := vaultBrowseFixture()

	browse, err := service.BrowseVault(context.Background(), "vault-1", CompendiumFilters{})
	if err != nil {
		t.Fatalf("browse vault: %v", err)
	}

	got := entryNames(browse.Entries)
	want := []string{"Aragorn's Sword", "Rations", "Traveler's Pack"}
	if len(got) != len(want) {
		t.Fatalf("entries = %v, want %v", got, want)
	}
	for index, name := range want {
		if got[index] != name {
			t.Fatalf("entries = %v, want %v (sorted by name)", got, want)
		}
	}
	if browse.TotalCount != 3 || browse.MatchCount != 3 {
		t.Fatalf("counts = %d/%d, want 3/3", browse.MatchCount, browse.TotalCount)
	}
}

func TestBrowseVaultIncludesItemsNestedInVaultContainers(t *testing.T) {
	t.Parallel()

	service := vaultBrowseFixture()

	browse, err := service.BrowseVault(context.Background(), "vault-1", CompendiumFilters{})
	if err != nil {
		t.Fatalf("browse vault: %v", err)
	}

	var rations *ItemEntry
	for index := range browse.Entries {
		if browse.Entries[index].Item.ID == "rations" {
			rations = &browse.Entries[index]
		}
	}
	if rations == nil {
		t.Fatalf("expected the nested rations to be listed, got %v", entryNames(browse.Entries))
	}
	if !rations.IsNested {
		t.Fatalf("expected the nested rations to be flagged as nested")
	}
	if rations.ContainerPath != "Traveler's Pack" {
		t.Fatalf("container path = %q, want %q", rations.ContainerPath, "Traveler's Pack")
	}
}

func TestBrowseVaultExcludesCompendiumAndOtherVaultItems(t *testing.T) {
	t.Parallel()

	service := vaultBrowseFixture()

	browse, err := service.BrowseVault(context.Background(), "vault-1", CompendiumFilters{})
	if err != nil {
		t.Fatalf("browse vault: %v", err)
	}

	for _, entry := range browse.Entries {
		if entry.Item.ID == "torch" {
			t.Fatalf("expected the compendium-owned torch to be excluded from vault-1's browse")
		}
		if entry.Item.ID == "elven-bow" {
			t.Fatalf("expected vault-2's elven bow to be excluded from vault-1's browse")
		}
	}
}

func TestBrowseVaultAppliesFiltersAndReportsCounts(t *testing.T) {
	t.Parallel()

	service := vaultBrowseFixture()

	browse, err := service.BrowseVault(context.Background(), "vault-1", CompendiumFilters{Category: "weapon"})
	if err != nil {
		t.Fatalf("browse vault: %v", err)
	}

	if got := entryNames(browse.Entries); len(got) != 1 || got[0] != "Aragorn's Sword" {
		t.Fatalf("entries = %v, want [Aragorn's Sword]", got)
	}
	if browse.MatchCount != 1 || browse.TotalCount != 3 {
		t.Fatalf("counts = %d/%d, want 1/3", browse.MatchCount, browse.TotalCount)
	}
}

func TestBrowseVaultBuildsFacetsFromVaultScopedItems(t *testing.T) {
	t.Parallel()

	service := vaultBrowseFixture()

	browse, err := service.BrowseVault(context.Background(), "vault-1", CompendiumFilters{Category: "weapon"})
	if err != nil {
		t.Fatalf("browse vault: %v", err)
	}

	// Facets describe every item vault-1 owns, not just the filtered result.
	if len(browse.Facets.WeaponDamageTypes) != 1 || browse.Facets.WeaponDamageTypes[0] != "slashing" {
		t.Fatalf("weapon damage facets = %v, want [slashing]", browse.Facets.WeaponDamageTypes)
	}
}

func TestBrowseVaultRejectsMalformedNumbers(t *testing.T) {
	t.Parallel()

	service := vaultBrowseFixture()

	if _, err := service.BrowseVault(context.Background(), "vault-1", CompendiumFilters{MinWeightLB: "heavy"}); err == nil {
		t.Fatalf("expected an error for a malformed weight bound")
	}
}

func TestBrowseVaultSortsOnTheValuesItDisplays(t *testing.T) {
	t.Parallel()

	service := vaultBrowseFixture()

	browse, err := service.BrowseVault(context.Background(), "vault-1", CompendiumFilters{SortBy: "weight"})
	if err != nil {
		t.Fatalf("browse vault: %v", err)
	}
	// Unit weights: rations 2lb, sword 3lb, pack 5lb.
	if got := entryNames(browse.Entries); got[0] != "Rations" {
		t.Fatalf("weight sort = %v, want the lightest per-unit entry first", got)
	}
}

func TestBrowseVaultRejectsUnknownVault(t *testing.T) {
	t.Parallel()

	service := vaultBrowseFixture()

	if _, err := service.BrowseVault(context.Background(), "does-not-exist", CompendiumFilters{}); err == nil {
		t.Fatalf("expected an error for an unknown vault")
	}
}

func TestItemEntryLocationLabelDistinguishesRootFromNested(t *testing.T) {
	t.Parallel()

	service := vaultBrowseFixture()

	browse, err := service.BrowseVault(context.Background(), "vault-1", CompendiumFilters{})
	if err != nil {
		t.Fatalf("browse vault: %v", err)
	}

	for _, entry := range browse.Entries {
		switch entry.Item.ID {
		case "rations":
			if entry.LocationLabel != "" {
				t.Fatalf("expected a nested entry to carry no root LocationLabel, got %q", entry.LocationLabel)
			}
		case "sword", "pack":
			if entry.LocationLabel != "Vault root" {
				t.Fatalf("expected a root entry LocationLabel of %q, got %q", "Vault root", entry.LocationLabel)
			}
		}
	}
}
