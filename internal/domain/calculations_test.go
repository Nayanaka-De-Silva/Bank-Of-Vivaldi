package domain

import "testing"

func TestComputePurseValueCP(t *testing.T) {
	purse := Purse{CP: 5, SP: 4, EP: 2, GP: 3, PP: 1}
	got := ComputePurseValueCP(purse)
	want := 5 + 40 + 100 + 300 + 1000
	if got != want {
		t.Fatalf("expected %d, got %d", want, got)
	}
}

func TestComputeCoinWeightHundredthsLB(t *testing.T) {
	purse := Purse{GP: 51}
	got := ComputeCoinWeightHundredthsLB(purse)
	if got != 102 {
		t.Fatalf("expected 102 hundredths lb, got %d", got)
	}
}

func TestGetEncumbranceStateOptionalThresholds(t *testing.T) {
	vault := Vault{
		StrengthScore:   10,
		EncumbranceMode: EncumbranceModeOptional,
	}

	if state := GetEncumbranceState(400, vault); state != EncumbranceStateNormal {
		t.Fatalf("expected normal, got %s", state)
	}
	if state := GetEncumbranceState(5001, vault); state != EncumbranceStateEncumbered {
		t.Fatalf("expected encumbered, got %s", state)
	}
	if state := GetEncumbranceState(10001, vault); state != EncumbranceStateHeavilyEncumbered {
		t.Fatalf("expected heavily encumbered, got %s", state)
	}
	if state := GetEncumbranceState(15001, vault); state != EncumbranceStateOverCapacity {
		t.Fatalf("expected over-capacity, got %s", state)
	}
}

func TestComputeNestedContainerAggregation(t *testing.T) {
	items := []Item{
		{
			ID:                 "backpack",
			Name:               "Backpack",
			Quantity:           1,
			WeightHundredthsLB: 500,
			IsContainer:        true,
			Details:            ItemDetails{Container: &ContainerDetails{MaxWeightHundredthsLB: 3000}},
			Location:           ItemLocation{Kind: LocationKindVaultRoot, OwnerVaultID: "vault-1"},
		},
		{
			ID:                 "rope",
			Name:               "Rope",
			Quantity:           2,
			WeightHundredthsLB: 1000,
			Location:           ItemLocation{Kind: LocationKindContainer, OwnerVaultID: "vault-1", ParentContainerItemID: "backpack"},
		},
		{
			ID:                 "pouch",
			Name:               "Pouch",
			Quantity:           1,
			WeightHundredthsLB: 100,
			IsContainer:        true,
			Details:            ItemDetails{Container: &ContainerDetails{MaxWeightHundredthsLB: 600}},
			Location:           ItemLocation{Kind: LocationKindContainer, OwnerVaultID: "vault-1", ParentContainerItemID: "backpack"},
		},
		{
			ID:                 "gem",
			Name:               "Gem",
			Quantity:           3,
			WeightHundredthsLB: 10,
			Location:           ItemLocation{Kind: LocationKindContainer, OwnerVaultID: "vault-1", ParentContainerItemID: "pouch"},
		},
	}

	byID, children := BuildItemIndexes(items)
	if got := ComputeSubtreeWeightHundredths("backpack", byID, children); got != 2630 {
		t.Fatalf("expected total subtree weight 2630, got %d", got)
	}
	if err := ValidateContainerCapacity(byID["backpack"], ComputeContainedWeightHundredths("backpack", byID, children)); err != nil {
		t.Fatalf("expected backpack capacity to pass, got %v", err)
	}
}

func TestValidateVaultCapacity(t *testing.T) {
	vault := Vault{CharacterName: "Vivaldi", StrengthScore: 10}
	if err := ValidateVaultCapacity(vault, 15001); err == nil {
		t.Fatalf("expected over-capacity validation error")
	}
}

func TestParseBulkItemInput(t *testing.T) {
	preview := ParseBulkItemInput("2x Rope | equipment | mundane | 10 | 100\nLantern", BulkDefaults{})
	if len(preview.Rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(preview.Rows))
	}
	if preview.Rows[0].Quantity != 2 || preview.Rows[0].Name != "Rope" {
		t.Fatalf("unexpected parsed first row: %+v", preview.Rows[0])
	}
	if preview.Rows[1].Quantity != 1 || preview.Rows[1].Name != "Lantern" {
		t.Fatalf("unexpected parsed second row: %+v", preview.Rows[1])
	}
}

func TestCloneItemDetailsIsDeep(t *testing.T) {
	src := ItemDetails{
		Container: &ContainerDetails{MaxWeightHundredthsLB: 500},
		Weapon:    &WeaponDetails{Properties: []string{"finesse", "light"}},
	}

	clone := CloneItemDetails(src)

	clone.Container.MaxWeightHundredthsLB = 9999
	clone.Weapon.Properties[0] = "heavy"

	if src.Container.MaxWeightHundredthsLB != 500 {
		t.Fatalf("CloneItemDetails shared Container pointer: source mutated to %d", src.Container.MaxWeightHundredthsLB)
	}
	if src.Weapon.Properties[0] != "finesse" {
		t.Fatalf("CloneItemDetails shared Weapon.Properties slice: source mutated to %q", src.Weapon.Properties[0])
	}
}

func TestItemsMergeable(t *testing.T) {
	a := Item{
		ID:                 "a",
		Name:               "Torch",
		Slug:               "torch",
		Category:           "equipment",
		Rarity:             RarityMundane,
		Quantity:           1,
		WeightHundredthsLB: 100,
		BaseValueCP:        10,
		IsStackable:        true,
		SourceKind:         SourceKindManual,
	}
	b := a
	b.ID = "b"
	if !ItemsMergeable(a, b) {
		t.Fatalf("expected items to be mergeable")
	}
}
