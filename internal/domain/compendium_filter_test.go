package domain

import (
	"reflect"
	"testing"
)

func intPtr(value int) *int    { return &value }
func boolPtr(value bool) *bool { return &value }

func armorItem() Item {
	item := Item{
		Name:               "Scale Mail",
		Category:           "armor",
		Rarity:             RarityMundane,
		WeightHundredthsLB: 4500,
		BaseValueCP:        5000,
	}
	item.Details.Armor = &ArmorDetails{
		ArmorCategory:       "medium",
		BaseAC:              14,
		DexModifierBehavior: "max-2",
		StealthDisadvantage: true,
	}
	return item
}

func weaponItem() Item {
	item := Item{
		Name:               "Longbow",
		Category:           "weapon",
		Rarity:             RarityMundane,
		WeightHundredthsLB: 200,
		BaseValueCP:        5000,
	}
	item.Details.Weapon = &WeaponDetails{
		WeaponClass: "martial-ranged",
		DamageDice:  "1d8",
		DamageType:  "piercing",
		Properties:  []string{"ammunition", "heavy", "two-handed"},
		NormalRange: 150,
		LongRange:   600,
	}
	return item
}

func TestCompendiumCriteriaEmptyMatchesEverything(t *testing.T) {
	t.Parallel()

	var criteria CompendiumCriteria
	if !criteria.Matches(armorItem()) {
		t.Fatalf("expected empty criteria to match any item")
	}
}

func TestCompendiumCriteriaFiltersByCategoryAndRarity(t *testing.T) {
	t.Parallel()

	item := armorItem()

	if (CompendiumCriteria{Category: "Armor"}).Matches(item) == false {
		t.Fatalf("expected category match to be case-insensitive")
	}
	if (CompendiumCriteria{Category: "weapon"}).Matches(item) {
		t.Fatalf("expected a non-matching category to exclude the item")
	}
	if !(CompendiumCriteria{Rarity: "mundane"}).Matches(item) {
		t.Fatalf("expected rarity match")
	}
	if (CompendiumCriteria{Rarity: "legendary"}).Matches(item) {
		t.Fatalf("expected a non-matching rarity to exclude the item")
	}
}

func TestCompendiumCriteriaFreeTextSearchesNameAndDescription(t *testing.T) {
	t.Parallel()

	item := armorItem()
	item.Description = "Interlocking metal rings sewn onto a leather backing."
	item.Subcategory = "body"

	for _, query := range []string{"scale", "SCALE MAIL", "leather backing", "body", "armor"} {
		if !(CompendiumCriteria{Query: query}).Matches(item) {
			t.Fatalf("expected query %q to match", query)
		}
	}
	if (CompendiumCriteria{Query: "longbow"}).Matches(item) {
		t.Fatalf("expected an unrelated query to exclude the item")
	}
}

func TestCompendiumCriteriaWeightAndValueBoundsAreInclusive(t *testing.T) {
	t.Parallel()

	item := armorItem() // 45 lb, 50 gp

	if !(CompendiumCriteria{MinWeightHundredthsLB: intPtr(4500), MaxWeightHundredthsLB: intPtr(4500)}).Matches(item) {
		t.Fatalf("expected weight bounds to be inclusive")
	}
	if (CompendiumCriteria{MinWeightHundredthsLB: intPtr(4501)}).Matches(item) {
		t.Fatalf("expected a heavier minimum to exclude the item")
	}
	if (CompendiumCriteria{MaxWeightHundredthsLB: intPtr(4499)}).Matches(item) {
		t.Fatalf("expected a lighter maximum to exclude the item")
	}
	if !(CompendiumCriteria{MinValueCP: intPtr(5000), MaxValueCP: intPtr(5000)}).Matches(item) {
		t.Fatalf("expected value bounds to be inclusive")
	}
	if (CompendiumCriteria{MinValueCP: intPtr(5001)}).Matches(item) {
		t.Fatalf("expected a higher minimum value to exclude the item")
	}
	if (CompendiumCriteria{MaxValueCP: intPtr(4999)}).Matches(item) {
		t.Fatalf("expected a lower maximum value to exclude the item")
	}
}

func TestCompendiumCriteriaFlagsAreTriState(t *testing.T) {
	t.Parallel()

	mundane := armorItem()
	magical := armorItem()
	magical.IsMagical = true
	magical.RequiresAttunement = true

	if !(CompendiumCriteria{}).Matches(mundane) || !(CompendiumCriteria{}).Matches(magical) {
		t.Fatalf("expected unset flags to match both items")
	}
	if !(CompendiumCriteria{IsMagical: boolPtr(true)}).Matches(magical) {
		t.Fatalf("expected magical filter to match a magical item")
	}
	if (CompendiumCriteria{IsMagical: boolPtr(true)}).Matches(mundane) {
		t.Fatalf("expected magical filter to exclude a mundane item")
	}
	if !(CompendiumCriteria{RequiresAttunement: boolPtr(false)}).Matches(mundane) {
		t.Fatalf("expected attunement=false filter to match a non-attuned item")
	}
}

func TestCompendiumCriteriaArmorGroup(t *testing.T) {
	t.Parallel()

	item := armorItem()

	criteria := CompendiumCriteria{Category: "armor"}
	criteria.Armor = ArmorCriteria{Category: "medium", MinBaseAC: intPtr(13), MaxBaseAC: intPtr(15)}
	if !criteria.Matches(item) {
		t.Fatalf("expected matching armor criteria to keep the item")
	}

	criteria.Armor.MinBaseAC = intPtr(16)
	if criteria.Matches(item) {
		t.Fatalf("expected an out-of-range base AC to exclude the item")
	}

	criteria = CompendiumCriteria{Category: "armor"}
	criteria.Armor = ArmorCriteria{Category: "heavy"}
	if criteria.Matches(item) {
		t.Fatalf("expected a non-matching armor category to exclude the item")
	}

	criteria = CompendiumCriteria{Category: "armor"}
	criteria.Armor = ArmorCriteria{StealthDisadvantage: boolPtr(false)}
	if criteria.Matches(item) {
		t.Fatalf("expected the stealth flag to exclude a stealth-disadvantaged item")
	}
}

func TestCompendiumCriteriaArmorGroupExcludesItemsWithoutArmorDetails(t *testing.T) {
	t.Parallel()

	bare := Item{Name: "Improvised Plate", Category: "armor"}

	criteria := CompendiumCriteria{Category: "armor"}
	if !criteria.Matches(bare) {
		t.Fatalf("expected an item without armor metadata to survive an empty armor filter")
	}

	criteria.Armor = ArmorCriteria{Category: "light"}
	if criteria.Matches(bare) {
		t.Fatalf("expected an item without armor metadata to fail a populated armor filter")
	}
}

func TestCompendiumCriteriaWeaponGroup(t *testing.T) {
	t.Parallel()

	item := weaponItem()

	criteria := CompendiumCriteria{Category: "weapon"}
	criteria.Weapon = WeaponCriteria{Category: "martial-ranged", DamageType: "Piercing", Property: "HEAVY"}
	if !criteria.Matches(item) {
		t.Fatalf("expected matching weapon criteria to keep the item")
	}

	criteria.Weapon.Property = "finesse"
	if criteria.Matches(item) {
		t.Fatalf("expected a missing weapon property to exclude the item")
	}

	criteria = CompendiumCriteria{Category: "weapon"}
	criteria.Weapon = WeaponCriteria{Category: "simple-melee"}
	if criteria.Matches(item) {
		t.Fatalf("expected a non-matching weapon category to exclude the item")
	}
}

func TestCompendiumCriteriaMetadataGroupsIgnoredForOtherCategories(t *testing.T) {
	t.Parallel()

	item := weaponItem()

	// An armor filter left over from a previous selection must not affect a weapon search.
	criteria := CompendiumCriteria{Category: "weapon"}
	criteria.Armor = ArmorCriteria{Category: "heavy", MinBaseAC: intPtr(18)}
	criteria.Weapon = WeaponCriteria{Category: "martial-ranged"}
	if !criteria.Matches(item) {
		t.Fatalf("expected criteria for a non-selected category to be ignored")
	}
}

func TestCompendiumCriteriaRemainingMetadataGroups(t *testing.T) {
	t.Parallel()

	container := Item{Name: "Chest", Category: "container", IsContainer: true}
	container.Details.Container = &ContainerDetails{MaxWeightHundredthsLB: 30000}
	containerCriteria := CompendiumCriteria{Category: "container"}
	containerCriteria.Container = ContainerCriteria{MinCapacityHundredthsLB: intPtr(20000)}
	if !containerCriteria.Matches(container) {
		t.Fatalf("expected container capacity filter to match")
	}
	containerCriteria.Container = ContainerCriteria{MaxCapacityHundredthsLB: intPtr(10000)}
	if containerCriteria.Matches(container) {
		t.Fatalf("expected a smaller capacity ceiling to exclude the chest")
	}

	tool := Item{Name: "Thieves' Tools", Category: "tool"}
	tool.Details.Tool = &ToolDetails{ToolCategory: "thieves"}
	toolCriteria := CompendiumCriteria{Category: "tool"}
	toolCriteria.Tool = ToolCriteria{Category: "thieves"}
	if !toolCriteria.Matches(tool) {
		t.Fatalf("expected tool category filter to match")
	}

	mount := Item{Name: "Riding Horse", Category: "mount"}
	mount.Details.Mount = &MountDetails{MountType: "horse", MovementSpeed: 60}
	mountCriteria := CompendiumCriteria{Category: "mount"}
	mountCriteria.Mount = MountCriteria{Type: "horse", MinSpeed: intPtr(50)}
	if !mountCriteria.Matches(mount) {
		t.Fatalf("expected mount filter to match")
	}
	mountCriteria.Mount.MinSpeed = intPtr(70)
	if mountCriteria.Matches(mount) {
		t.Fatalf("expected a faster minimum speed to exclude the mount")
	}

	vehicle := Item{Name: "Rowboat", Category: "vehicle"}
	vehicle.Details.Vehicle = &VehicleDetails{VehicleType: "water", MovementSpeed: 15}
	vehicleCriteria := CompendiumCriteria{Category: "vehicle"}
	vehicleCriteria.Vehicle = VehicleCriteria{Type: "water", MaxSpeed: intPtr(20)}
	if !vehicleCriteria.Matches(vehicle) {
		t.Fatalf("expected vehicle filter to match")
	}

	treasure := Item{Name: "Amber Gem", Category: "treasure"}
	treasure.Details.Treasure = &TreasureDetails{TreasureKind: "gem"}
	treasureCriteria := CompendiumCriteria{Category: "treasure"}
	treasureCriteria.Treasure = TreasureCriteria{Kind: "gem"}
	if !treasureCriteria.Matches(treasure) {
		t.Fatalf("expected treasure filter to match")
	}
	treasureCriteria.Treasure = TreasureCriteria{Kind: "art"}
	if treasureCriteria.Matches(treasure) {
		t.Fatalf("expected a non-matching treasure kind to exclude the item")
	}
}

func TestWeaponCriteriaMatchesTwoHandedStoredWithSpaceFormat(t *testing.T) {
	t.Parallel()

	// Legacy items may have "Two Handed" stored instead of the canonical "two-handed"
	// slug. EqualWeaponProperty must bridge that gap so the filter still matches.
	item := Item{Name: "Greatsword", Category: "weapon"}
	item.Details.Weapon = &WeaponDetails{
		WeaponClass: "martial-melee",
		Properties:  []string{"Two Handed"},
	}

	criteria := CompendiumCriteria{Category: "weapon"}
	criteria.Weapon = WeaponCriteria{Property: "two-handed"}
	if !criteria.Matches(item) {
		t.Fatalf("expected item with property %q to match filter %q via EqualWeaponProperty", "Two Handed", "two-handed")
	}
}

func TestBuildCompendiumFacetsDedupesAndSorts(t *testing.T) {
	t.Parallel()

	heavy := armorItem()
	heavy.Details.Armor = &ArmorDetails{ArmorCategory: "Heavy", DexModifierBehavior: "none"}

	duplicate := armorItem()

	sword := weaponItem()
	sword.Details.Weapon = &WeaponDetails{
		WeaponClass: "martial-melee",
		DamageType:  "Slashing",
		Properties:  []string{"versatile", "heavy"},
	}

	treasure := Item{Name: "Amber Gem", Category: "treasure"}
	treasure.Details.Treasure = &TreasureDetails{TreasureKind: "gem"}

	facets := BuildCompendiumFacets([]Item{heavy, duplicate, armorItem(), weaponItem(), sword, treasure})

	if want := []string{"Heavy", "medium"}; !reflect.DeepEqual(facets.ArmorCategories, want) {
		t.Fatalf("armor categories = %v, want %v", facets.ArmorCategories, want)
	}
	if want := []string{"max-2", "none"}; !reflect.DeepEqual(facets.ArmorDexBehaviors, want) {
		t.Fatalf("armor dex behaviors = %v, want %v", facets.ArmorDexBehaviors, want)
	}
	if want := []string{"piercing", "Slashing"}; !reflect.DeepEqual(facets.WeaponDamageTypes, want) {
		t.Fatalf("weapon damage types = %v, want %v", facets.WeaponDamageTypes, want)
	}
	if want := []string{"ammunition", "heavy", "two-handed", "versatile"}; !reflect.DeepEqual(facets.WeaponProperties, want) {
		t.Fatalf("weapon properties = %v, want %v", facets.WeaponProperties, want)
	}
	if want := []string{"gem"}; !reflect.DeepEqual(facets.TreasureKinds, want) {
		t.Fatalf("treasure kinds = %v, want %v", facets.TreasureKinds, want)
	}
	if len(facets.ToolCategories) != 0 {
		t.Fatalf("expected no tool facets, got %v", facets.ToolCategories)
	}
}
