package domain

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

func ComputePurseValueCP(p Purse) int {
	return p.CP + (p.SP * 10) + (p.EP * 50) + (p.GP * 100) + (p.PP * 1000)
}

func ComputeCoinWeightHundredthsLB(p Purse) int {
	coins := p.TotalCoins()
	if coins == 0 {
		return 0
	}
	return (coins*100 + 49) / 50
}

func GetEncumbranceState(totalWeightHundredths int, vault Vault) EncumbranceState {
	standardCap := vault.CarryCapacityHundredthsLB()
	if totalWeightHundredths > standardCap {
		return EncumbranceStateOverCapacity
	}

	if vault.EncumbranceMode != EncumbranceModeOptional {
		return EncumbranceStateNormal
	}

	encumbered := (vault.StrengthScore*5 + vault.CarryModifierLB) * 100
	heavy := (vault.StrengthScore*10 + vault.CarryModifierLB) * 100
	switch {
	case totalWeightHundredths > heavy:
		return EncumbranceStateHeavilyEncumbered
	case totalWeightHundredths > encumbered:
		return EncumbranceStateEncumbered
	default:
		return EncumbranceStateNormal
	}
}

func ValidateVaultCapacity(vault Vault, totalWeightHundredths int) error {
	if totalWeightHundredths > vault.CarryCapacityHundredthsLB() {
		return fmt.Errorf("vault %q would exceed carrying capacity", vault.CharacterName)
	}
	return nil
}

func ValidateContainerCapacity(container Item, containedWeightHundredths int) error {
	maxWeight := container.ContainerMaxWeightHundredthsLB()
	if maxWeight == 0 {
		return nil
	}
	if containedWeightHundredths > maxWeight {
		return fmt.Errorf("container %q would exceed capacity", container.Name)
	}
	return nil
}

func BuildItemIndexes(items []Item) (map[string]Item, map[string][]Item) {
	byID := make(map[string]Item, len(items))
	children := map[string][]Item{}
	for _, item := range items {
		byID[item.ID] = item
		if parentID := item.Location.ParentContainerItemID; parentID != "" {
			children[parentID] = append(children[parentID], item)
		}
	}

	for parentID := range children {
		sort.Slice(children[parentID], func(i, j int) bool {
			return strings.ToLower(children[parentID][i].Name) < strings.ToLower(children[parentID][j].Name)
		})
	}

	return byID, children
}

func ComputeSubtreeWeightHundredths(itemID string, byID map[string]Item, children map[string][]Item) int {
	item, ok := byID[itemID]
	if !ok {
		return 0
	}

	total := item.TotalWeightHundredthsLB()
	for _, child := range children[itemID] {
		total += ComputeSubtreeWeightHundredths(child.ID, byID, children)
	}
	return total
}

func ComputeSubtreeValueCP(itemID string, byID map[string]Item, children map[string][]Item) int {
	item, ok := byID[itemID]
	if !ok {
		return 0
	}

	total := item.TotalValueCP()
	for _, child := range children[itemID] {
		total += ComputeSubtreeValueCP(child.ID, byID, children)
	}
	return total
}

func ComputeContainedWeightHundredths(containerID string, byID map[string]Item, children map[string][]Item) int {
	total := 0
	for _, child := range children[containerID] {
		total += ComputeSubtreeWeightHundredths(child.ID, byID, children)
	}
	return total
}

func ComputeContainedValueCP(containerID string, byID map[string]Item, children map[string][]Item) int {
	total := 0
	for _, child := range children[containerID] {
		total += ComputeSubtreeValueCP(child.ID, byID, children)
	}
	return total
}

func ComputeVaultItemWeightHundredths(items []Item, vaultID string) int {
	byID, children := BuildItemIndexes(items)
	total := 0
	for _, item := range items {
		if item.Location.Kind == LocationKindVaultRoot && item.Location.OwnerVaultID == vaultID {
			total += ComputeSubtreeWeightHundredths(item.ID, byID, children)
		}
	}
	return total
}

func ComputeVaultItemValueCP(items []Item, vaultID string) int {
	byID, children := BuildItemIndexes(items)
	total := 0
	for _, item := range items {
		if item.Location.Kind == LocationKindVaultRoot && item.Location.OwnerVaultID == vaultID {
			total += ComputeSubtreeValueCP(item.ID, byID, children)
		}
	}
	return total
}

func ComputeCompendiumWeightHundredths(items []Item) int {
	byID, children := BuildItemIndexes(items)
	total := 0
	for _, item := range items {
		if item.Location.Kind == LocationKindCompendiumRoot {
			total += ComputeSubtreeWeightHundredths(item.ID, byID, children)
		}
	}
	return total
}

func ComputeCompendiumValueCP(items []Item) int {
	byID, children := BuildItemIndexes(items)
	total := 0
	for _, item := range items {
		if item.Location.Kind == LocationKindCompendiumRoot {
			total += ComputeSubtreeValueCP(item.ID, byID, children)
		}
	}
	return total
}

func ItemsMergeable(a, b Item) bool {
	if a.Name != b.Name ||
		a.Slug != b.Slug ||
		a.Description != b.Description ||
		a.Category != b.Category ||
		a.Subcategory != b.Subcategory ||
		a.Rarity != b.Rarity ||
		a.WeightHundredthsLB != b.WeightHundredthsLB ||
		a.BaseValueCP != b.BaseValueCP ||
		a.IsContainer != b.IsContainer ||
		a.IsStackable != b.IsStackable ||
		a.IsEquipped != b.IsEquipped ||
		a.IsMagical != b.IsMagical ||
		a.RequiresAttunement != b.RequiresAttunement ||
		a.SourceKind != b.SourceKind {
		return false
	}

	return reflect.DeepEqual(normalizeDetails(a.Details), normalizeDetails(b.Details))
}

func CloneItems(items []Item) []Item {
	cloned := make([]Item, len(items))
	copy(cloned, items)
	return cloned
}

// CloneItemDetails returns a deep copy of d via JSON round-trip, so that any
// pointer field added later is automatically handled without changing this code.
func CloneItemDetails(d ItemDetails) ItemDetails {
	return normalizeDetails(d)
}

func normalizeDetails(details ItemDetails) ItemDetails {
	bytes, err := json.Marshal(details)
	if err != nil {
		return details
	}

	var normalized ItemDetails
	if err := json.Unmarshal(bytes, &normalized); err != nil {
		return details
	}
	return normalized
}
