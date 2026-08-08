package application

import (
	"context"
	"testing"

	"bank-of-vivaldi/internal/domain"
)

func compendiumFixture() *Service {
	store := newFakeStore()
	store.vaults["vault-1"] = domain.Vault{ID: "vault-1", CharacterName: "Aragorn", StrengthScore: 16}

	chest := domain.Item{
		ID:                 "chest",
		Name:               "Oak Chest",
		Category:           "container",
		Rarity:             domain.RarityMundane,
		WeightHundredthsLB: 2500,
		BaseValueCP:        500,
		Quantity:           1,
		IsContainer:        true,
		Location:           domain.ItemLocation{Kind: domain.LocationKindCompendiumRoot},
	}
	chest.Details.Container = &domain.ContainerDetails{MaxWeightHundredthsLB: 30000}

	scaleMail := domain.Item{
		ID:                 "scale",
		Name:               "Scale Mail",
		Category:           "armor",
		Rarity:             domain.RarityMundane,
		WeightHundredthsLB: 4500,
		BaseValueCP:        5000,
		Quantity:           1,
		Location:           domain.ItemLocation{Kind: domain.LocationKindCompendiumRoot},
	}
	scaleMail.Details.Armor = &domain.ArmorDetails{ArmorCategory: "medium", BaseAC: 14, StealthDisadvantage: true}

	// Nested inside the compendium chest: no owning vault, container location kind.
	longbow := domain.Item{
		ID:                 "longbow",
		Name:               "Longbow",
		Category:           "weapon",
		Rarity:             domain.RarityMundane,
		WeightHundredthsLB: 200,
		BaseValueCP:        5000,
		Quantity:           1,
		Location:           domain.ItemLocation{Kind: domain.LocationKindContainer, ParentContainerItemID: "chest"},
	}
	longbow.Details.Weapon = &domain.WeaponDetails{WeaponClass: "martial-ranged", DamageType: "piercing"}

	// Owned by a vault: must never appear in the compendium browser.
	vaultSword := domain.Item{
		ID:                 "sword",
		Name:               "Aragorn's Sword",
		Category:           "weapon",
		Rarity:             domain.RarityRare,
		WeightHundredthsLB: 300,
		BaseValueCP:        150000,
		Quantity:           1,
		Location:           domain.ItemLocation{Kind: domain.LocationKindVaultRoot, OwnerVaultID: "vault-1"},
	}

	for _, item := range []domain.Item{chest, scaleMail, longbow, vaultSword} {
		store.items[item.ID] = item
	}

	return NewService(store)
}

func entryNames(entries []CompendiumEntry) []string {
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Item.Name)
	}
	return names
}

func TestBrowseCompendiumScopesToCompendiumItems(t *testing.T) {
	t.Parallel()

	service := compendiumFixture()

	browse, err := service.BrowseCompendium(context.Background(), CompendiumFilters{})
	if err != nil {
		t.Fatalf("browse compendium: %v", err)
	}

	got := entryNames(browse.Entries)
	want := []string{"Longbow", "Oak Chest", "Scale Mail"}
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

func TestBrowseCompendiumIncludesNestedItemsWithContainerPath(t *testing.T) {
	t.Parallel()

	service := compendiumFixture()

	browse, err := service.BrowseCompendium(context.Background(), CompendiumFilters{})
	if err != nil {
		t.Fatalf("browse compendium: %v", err)
	}

	var longbow *CompendiumEntry
	for index := range browse.Entries {
		if browse.Entries[index].Item.ID == "longbow" {
			longbow = &browse.Entries[index]
		}
	}
	if longbow == nil {
		t.Fatalf("expected the nested longbow to be listed, got %v", entryNames(browse.Entries))
	}
	if !longbow.IsNested {
		t.Fatalf("expected the nested longbow to be flagged as nested")
	}
	if longbow.ContainerPath != "Oak Chest" {
		t.Fatalf("container path = %q, want %q", longbow.ContainerPath, "Oak Chest")
	}
}

func TestBrowseCompendiumAppliesFiltersAndReportsCounts(t *testing.T) {
	t.Parallel()

	service := compendiumFixture()

	browse, err := service.BrowseCompendium(context.Background(), CompendiumFilters{
		Category:      "armor",
		ArmorCategory: "medium",
		ArmorMinAC:    "14",
	})
	if err != nil {
		t.Fatalf("browse compendium: %v", err)
	}

	if got := entryNames(browse.Entries); len(got) != 1 || got[0] != "Scale Mail" {
		t.Fatalf("entries = %v, want [Scale Mail]", got)
	}
	if browse.MatchCount != 1 || browse.TotalCount != 3 {
		t.Fatalf("counts = %d/%d, want 1/3", browse.MatchCount, browse.TotalCount)
	}
}

func TestBrowseCompendiumWeightAndValueBoundsUseDecimalInput(t *testing.T) {
	t.Parallel()

	service := compendiumFixture()

	browse, err := service.BrowseCompendium(context.Background(), CompendiumFilters{MaxWeightLB: "25"})
	if err != nil {
		t.Fatalf("browse compendium: %v", err)
	}
	if got := entryNames(browse.Entries); len(got) != 2 {
		t.Fatalf("entries = %v, want the two items at or under 25 lb", got)
	}

	// Values are entered in gold pieces; the longbow and scale mail are both 50 gp.
	browse, err = service.BrowseCompendium(context.Background(), CompendiumFilters{MinValueGP: "50"})
	if err != nil {
		t.Fatalf("browse compendium: %v", err)
	}
	if got := entryNames(browse.Entries); len(got) != 2 {
		t.Fatalf("entries = %v, want the two 50 gp items", got)
	}
}

func TestBrowseCompendiumSorting(t *testing.T) {
	t.Parallel()

	service := compendiumFixture()

	browse, err := service.BrowseCompendium(context.Background(), CompendiumFilters{SortBy: "weight"})
	if err != nil {
		t.Fatalf("browse compendium: %v", err)
	}
	if got := entryNames(browse.Entries); got[0] != "Longbow" || got[len(got)-1] != "Scale Mail" {
		t.Fatalf("weight sort = %v, want lightest first", got)
	}

	browse, err = service.BrowseCompendium(context.Background(), CompendiumFilters{SortBy: "value"})
	if err != nil {
		t.Fatalf("browse compendium: %v", err)
	}
	if got := entryNames(browse.Entries); got[0] != "Oak Chest" {
		t.Fatalf("value sort = %v, want cheapest first", got)
	}
}

func TestBrowseCompendiumBuildsFacetsFromScopedItems(t *testing.T) {
	t.Parallel()

	service := compendiumFixture()

	browse, err := service.BrowseCompendium(context.Background(), CompendiumFilters{Category: "armor"})
	if err != nil {
		t.Fatalf("browse compendium: %v", err)
	}

	// Facets describe the whole compendium, not just the filtered result, so the
	// dropdowns stay usable after a filter narrows the list.
	if len(browse.Facets.ArmorCategories) != 1 || browse.Facets.ArmorCategories[0] != "medium" {
		t.Fatalf("armor facets = %v, want [medium]", browse.Facets.ArmorCategories)
	}
	if len(browse.Facets.WeaponDamageTypes) != 1 || browse.Facets.WeaponDamageTypes[0] != "piercing" {
		t.Fatalf("weapon damage facets = %v, want [piercing]", browse.Facets.WeaponDamageTypes)
	}
}

func TestBrowseCompendiumRejectsMalformedNumbers(t *testing.T) {
	t.Parallel()

	service := compendiumFixture()

	if _, err := service.BrowseCompendium(context.Background(), CompendiumFilters{MinWeightLB: "heavy"}); err == nil {
		t.Fatalf("expected an error for a malformed weight bound")
	}
	if _, err := service.BrowseCompendium(context.Background(), CompendiumFilters{ArmorMinAC: "high"}); err == nil {
		t.Fatalf("expected an error for a malformed armor AC bound")
	}
}

func TestCompendiumFiltersCriteriaParsesTriStateFlags(t *testing.T) {
	t.Parallel()

	criteria, err := CompendiumFilters{Magical: "yes", Attunement: "no"}.Criteria()
	if err != nil {
		t.Fatalf("criteria: %v", err)
	}
	if criteria.IsMagical == nil || !*criteria.IsMagical {
		t.Fatalf("expected magical=yes to parse as true")
	}
	if criteria.RequiresAttunement == nil || *criteria.RequiresAttunement {
		t.Fatalf("expected attunement=no to parse as false")
	}

	criteria, err = CompendiumFilters{}.Criteria()
	if err != nil {
		t.Fatalf("criteria: %v", err)
	}
	if criteria.IsMagical != nil || criteria.RequiresAttunement != nil {
		t.Fatalf("expected blank flags to stay unset")
	}
}

func TestCompendiumFiltersActiveCategoryHelpers(t *testing.T) {
	t.Parallel()

	filters := CompendiumFilters{Category: "Armor"}
	if !filters.ShowsGroup("armor") {
		t.Fatalf("expected the armor group to be shown for the armor category")
	}
	if filters.ShowsGroup("weapon") {
		t.Fatalf("expected the weapon group to be hidden for the armor category")
	}
	if (CompendiumFilters{}).ShowsGroup("armor") {
		t.Fatalf("expected no group to be shown when no category is selected")
	}
}

func TestCompendiumFiltersCountActiveFields(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		filters CompendiumFilters
		want    int
	}{
		{"empty", CompendiumFilters{}, 0},
		{"blank strings are not filters", CompendiumFilters{Query: "   ", Rarity: ""}, 0},
		{"default sort is not a filter", CompendiumFilters{SortBy: "name"}, 0},
		{"explicit sort counts", CompendiumFilters{SortBy: "value"}, 1},
		{"query only", CompendiumFilters{Query: "rope"}, 1},
		{"base plus category metadata", CompendiumFilters{
			Query:         "rope",
			Rarity:        "common",
			ArmorCategory: "heavy",
		}, 3},
		{"tri-state any is not a filter", CompendiumFilters{Magical: "any"}, 0},
		{"tri-state yes counts", CompendiumFilters{Magical: "yes"}, 1},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if got := testCase.filters.ActiveFilterCount(); got != testCase.want {
				t.Fatalf("ActiveFilterCount() = %d, want %d", got, testCase.want)
			}
			if got := testCase.filters.HasActiveFilters(); got != (testCase.want > 0) {
				t.Fatalf("HasActiveFilters() = %v, want %v", got, testCase.want > 0)
			}
		})
	}
}

func TestBrowseCompendiumSortsOnTheValuesItDisplays(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	// A cheap, light stack whose totals outrank its per-item figures: the compendium
	// shows and filters on unit weight/value, so it must sort on them too.
	store.items["gems"] = domain.Item{
		ID:                 "gems",
		Name:               "Amber Gems",
		Category:           "treasure",
		Rarity:             domain.RarityUncommon,
		WeightHundredthsLB: 100,
		BaseValueCP:        10000,
		Quantity:           100,
		Location:           domain.ItemLocation{Kind: domain.LocationKindCompendiumRoot},
	}
	store.items["plate"] = domain.Item{
		ID:                 "plate",
		Name:               "Plate Armor",
		Category:           "armor",
		Rarity:             domain.RarityMundane,
		WeightHundredthsLB: 6500,
		BaseValueCP:        150000,
		Quantity:           1,
		Location:           domain.ItemLocation{Kind: domain.LocationKindCompendiumRoot},
	}
	service := NewService(store)

	browse, err := service.BrowseCompendium(context.Background(), CompendiumFilters{SortBy: "value"})
	if err != nil {
		t.Fatalf("browse compendium: %v", err)
	}
	// Unit values: gems 100 gp, plate 1500 gp. Stack totals would flip this order.
	if got := entryNames(browse.Entries); got[0] != "Amber Gems" {
		t.Fatalf("value sort = %v, want the cheaper per-item entry first", got)
	}

	browse, err = service.BrowseCompendium(context.Background(), CompendiumFilters{SortBy: "weight"})
	if err != nil {
		t.Fatalf("browse compendium: %v", err)
	}
	// Unit weights: gems 1 lb, plate 65 lb. Stack totals would flip this order.
	if got := entryNames(browse.Entries); got[0] != "Amber Gems" {
		t.Fatalf("weight sort = %v, want the lighter per-item entry first", got)
	}
}
