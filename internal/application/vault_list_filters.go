package application

import (
	"sort"
	"strings"
)

// VaultListFilters carries raw form/query values for the vault summary list.
// All fields are unparsed strings so the template can repopulate form controls
// exactly as submitted, mirroring CompendiumFilters.
type VaultListFilters struct {
	Query         string
	Kind          string // "pc" or "npc"
	MinCarryLB    string
	MaxCarryLB    string
	MinCapacityLB string
	MaxCapacityLB string
	MinItems      string
	MaxItems      string
	State         string // encumbrance-state slug, e.g. "encumbered"
	SortBy        string // name|type|carry|capacity|items|value
}

// ActiveFilterCount counts how many filter fields are actually narrowing the
// result set. SortBy is a display-order control and is never counted.
func (f VaultListFilters) ActiveFilterCount() int {
	count := 0
	for _, value := range []string{
		f.Query, f.Kind,
		f.MinCarryLB, f.MaxCarryLB,
		f.MinCapacityLB, f.MaxCapacityLB,
		f.MinItems, f.MaxItems,
		f.State,
	} {
		if strings.TrimSpace(value) != "" {
			count++
		}
	}
	return count
}

// HasActiveFilters reports whether any filter is narrowing the result set.
func (f VaultListFilters) HasActiveFilters() bool {
	return f.ActiveFilterCount() > 0
}

// Apply parses bounds and returns the filtered, sorted slice of summaries.
func (f VaultListFilters) Apply(summaries []VaultSummary) ([]VaultSummary, error) {
	minCarry, err := parseOptionalHundredths(f.MinCarryLB, "minimum carry weight")
	if err != nil {
		return nil, err
	}
	maxCarry, err := parseOptionalHundredths(f.MaxCarryLB, "maximum carry weight")
	if err != nil {
		return nil, err
	}
	minCap, err := parseOptionalHundredths(f.MinCapacityLB, "minimum capacity")
	if err != nil {
		return nil, err
	}
	maxCap, err := parseOptionalHundredths(f.MaxCapacityLB, "maximum capacity")
	if err != nil {
		return nil, err
	}
	minItems, err := parseOptionalInt(f.MinItems, "minimum items")
	if err != nil {
		return nil, err
	}
	maxItems, err := parseOptionalInt(f.MaxItems, "maximum items")
	if err != nil {
		return nil, err
	}

	query := strings.ToLower(strings.TrimSpace(f.Query))
	kindFilter := strings.ToLower(strings.TrimSpace(f.Kind))
	stateFilter := strings.ToLower(strings.TrimSpace(f.State))

	result := make([]VaultSummary, 0, len(summaries))
	for _, s := range summaries {
		if query != "" && !strings.Contains(strings.ToLower(s.Vault.CharacterName), query) {
			continue
		}
		if kindFilter != "" && string(s.Vault.Kind) != kindFilter {
			continue
		}
		if stateFilter != "" && string(s.EncumbranceState) != stateFilter {
			continue
		}
		if minCarry != nil && s.TotalCarryWeightHundredths < *minCarry {
			continue
		}
		if maxCarry != nil && s.TotalCarryWeightHundredths > *maxCarry {
			continue
		}
		if minCap != nil && s.MaxCarryWeightHundredths < *minCap {
			continue
		}
		if maxCap != nil && s.MaxCarryWeightHundredths > *maxCap {
			continue
		}
		if minItems != nil && s.ItemCount < *minItems {
			continue
		}
		if maxItems != nil && s.ItemCount > *maxItems {
			continue
		}
		result = append(result, s)
	}

	sort.SliceStable(result, func(i, j int) bool {
		return vaultSummaryLess(result[i], result[j], f.SortBy)
	})

	return result, nil
}

// vaultSummaryLess defines sort order for vault summaries. The default (empty
// SortBy or "name") mirrors the alphabetical order summarizeVaults already
// applies, so the apparent order doesn't change when no sort is requested.
func vaultSummaryLess(a, b VaultSummary, sortBy string) bool {
	switch sortBy {
	case "type":
		if a.Vault.Kind != b.Vault.Kind {
			return string(a.Vault.Kind) < string(b.Vault.Kind)
		}
		return strings.ToLower(a.Vault.CharacterName) < strings.ToLower(b.Vault.CharacterName)
	case "carry":
		return a.TotalCarryWeightHundredths < b.TotalCarryWeightHundredths
	case "capacity":
		return a.MaxCarryWeightHundredths < b.MaxCarryWeightHundredths
	case "items":
		return a.ItemCount < b.ItemCount
	case "value":
		return a.TotalCombinedValueCP < b.TotalCombinedValueCP
	default: // "name" or empty
		return strings.ToLower(a.Vault.CharacterName) < strings.ToLower(b.Vault.CharacterName)
	}
}
