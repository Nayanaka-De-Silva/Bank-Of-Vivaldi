package application

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"bank-of-vivaldi/internal/domain"
)

// CompendiumFilters carries the raw compendium browse form values. Keeping the
// unparsed strings lets the template repopulate the form exactly as submitted,
// mirroring how SearchFilters serves the /search page.
type CompendiumFilters struct {
	Query       string
	Category    string
	Rarity      string
	MinWeightLB string
	MaxWeightLB string
	MinValueGP  string
	MaxValueGP  string
	Magical     string
	Attunement  string
	SortBy      string

	ArmorCategory    string
	ArmorDexBehavior string
	ArmorMinAC       string
	ArmorMaxAC       string
	ArmorStealth     string

	WeaponCategory   string
	WeaponDamageType string
	WeaponProperty   string

	ContainerMinCapacityLB string
	ContainerMaxCapacityLB string

	ToolCategory string

	MountType     string
	MountMinSpeed string
	MountMaxSpeed string

	VehicleType     string
	VehicleMinSpeed string
	VehicleMaxSpeed string

	TreasureKind string
}

// ActiveCategory returns the normalized selected category, or "" when the DM has
// not narrowed to one.
func (f CompendiumFilters) ActiveCategory() string {
	return strings.ToLower(strings.TrimSpace(f.Category))
}

// ShowsGroup reports whether a metadata filter group applies to the current
// category selection, so the template can render inactive groups hidden.
func (f CompendiumFilters) ShowsGroup(group string) bool {
	active := f.ActiveCategory()
	return active != "" && active == strings.ToLower(strings.TrimSpace(group))
}

// Criteria parses the raw form values into a domain predicate, reporting the first
// malformed numeric input rather than silently ignoring it.
func (f CompendiumFilters) Criteria() (domain.CompendiumCriteria, error) {
	criteria := domain.CompendiumCriteria{
		Query:              strings.TrimSpace(f.Query),
		Category:           f.ActiveCategory(),
		Rarity:             strings.ToLower(strings.TrimSpace(f.Rarity)),
		IsMagical:          parseTriState(f.Magical),
		RequiresAttunement: parseTriState(f.Attunement),
	}

	var err error
	if criteria.MinWeightHundredthsLB, err = parseOptionalHundredths(f.MinWeightLB, "minimum weight"); err != nil {
		return domain.CompendiumCriteria{}, err
	}
	if criteria.MaxWeightHundredthsLB, err = parseOptionalHundredths(f.MaxWeightLB, "maximum weight"); err != nil {
		return domain.CompendiumCriteria{}, err
	}
	// Values are entered in gold pieces and stored in copper; 1 gp = 100 cp, so the
	// same decimal-to-hundredths parser applies.
	if criteria.MinValueCP, err = parseOptionalHundredths(f.MinValueGP, "minimum value"); err != nil {
		return domain.CompendiumCriteria{}, err
	}
	if criteria.MaxValueCP, err = parseOptionalHundredths(f.MaxValueGP, "maximum value"); err != nil {
		return domain.CompendiumCriteria{}, err
	}

	criteria.Armor = domain.ArmorCriteria{
		Category:            strings.TrimSpace(f.ArmorCategory),
		DexBehavior:         strings.TrimSpace(f.ArmorDexBehavior),
		StealthDisadvantage: parseTriState(f.ArmorStealth),
	}
	if criteria.Armor.MinBaseAC, err = parseOptionalInt(f.ArmorMinAC, "minimum base AC"); err != nil {
		return domain.CompendiumCriteria{}, err
	}
	if criteria.Armor.MaxBaseAC, err = parseOptionalInt(f.ArmorMaxAC, "maximum base AC"); err != nil {
		return domain.CompendiumCriteria{}, err
	}

	criteria.Weapon = domain.WeaponCriteria{
		Category:   strings.TrimSpace(f.WeaponCategory),
		DamageType: strings.TrimSpace(f.WeaponDamageType),
		Property:   strings.TrimSpace(f.WeaponProperty),
	}

	if criteria.Container.MinCapacityHundredthsLB, err = parseOptionalHundredths(f.ContainerMinCapacityLB, "minimum capacity"); err != nil {
		return domain.CompendiumCriteria{}, err
	}
	if criteria.Container.MaxCapacityHundredthsLB, err = parseOptionalHundredths(f.ContainerMaxCapacityLB, "maximum capacity"); err != nil {
		return domain.CompendiumCriteria{}, err
	}

	criteria.Tool = domain.ToolCriteria{Category: strings.TrimSpace(f.ToolCategory)}

	criteria.Mount = domain.MountCriteria{Type: strings.TrimSpace(f.MountType)}
	if criteria.Mount.MinSpeed, err = parseOptionalInt(f.MountMinSpeed, "minimum mount speed"); err != nil {
		return domain.CompendiumCriteria{}, err
	}
	if criteria.Mount.MaxSpeed, err = parseOptionalInt(f.MountMaxSpeed, "maximum mount speed"); err != nil {
		return domain.CompendiumCriteria{}, err
	}

	criteria.Vehicle = domain.VehicleCriteria{Type: strings.TrimSpace(f.VehicleType)}
	if criteria.Vehicle.MinSpeed, err = parseOptionalInt(f.VehicleMinSpeed, "minimum vehicle speed"); err != nil {
		return domain.CompendiumCriteria{}, err
	}
	if criteria.Vehicle.MaxSpeed, err = parseOptionalInt(f.VehicleMaxSpeed, "maximum vehicle speed"); err != nil {
		return domain.CompendiumCriteria{}, err
	}

	criteria.Treasure = domain.TreasureCriteria{Kind: strings.TrimSpace(f.TreasureKind)}

	return criteria, nil
}

// CompendiumEntry is one row or tile in the compendium browser.
type CompendiumEntry struct {
	Item           domain.Item
	ContainerPath  string
	IsNested       bool
	UnitWeightLB   string
	UnitValueText  string
	TotalWeightLB  string
	TotalValueText string
}

// CompendiumBrowse is the rendered state of the compendium browser.
type CompendiumBrowse struct {
	Settings   domain.AppSettings
	Entries    []CompendiumEntry
	Facets     domain.CompendiumFacets
	TotalCount int
	MatchCount int
}

// BrowseCompendium lists every compendium-held item, including items nested inside
// compendium containers, narrowed by the supplied filters.
func (s *Service) BrowseCompendium(ctx context.Context, filters CompendiumFilters) (CompendiumBrowse, error) {
	criteria, err := filters.Criteria()
	if err != nil {
		return CompendiumBrowse{}, err
	}

	settings, err := s.store.GetAppSettings(ctx)
	if err != nil {
		return CompendiumBrowse{}, err
	}

	allItems, err := s.store.ListItems(ctx)
	if err != nil {
		return CompendiumBrowse{}, err
	}

	scoped := compendiumScopedItems(allItems)
	entries := make([]CompendiumEntry, 0, len(scoped))
	for _, item := range scoped {
		if !criteria.Matches(item) {
			continue
		}
		entries = append(entries, CompendiumEntry{
			Item:           item,
			ContainerPath:  buildContainerPath(item, allItems),
			IsNested:       item.Location.ParentContainerItemID != "",
			UnitWeightLB:   domain.FormatWeightHundredths(item.UnitWeightHundredthsLB()),
			UnitValueText:  domain.FormatCopperAsGold(item.BaseValueCP),
			TotalWeightLB:  domain.FormatWeightHundredths(item.TotalWeightHundredthsLB()),
			TotalValueText: domain.FormatCopperAsGold(item.TotalValueCP()),
		})
	}

	sort.SliceStable(entries, func(i, j int) bool {
		return compendiumEntryLess(entries[i].Item, entries[j].Item, filters.SortBy)
	})

	return CompendiumBrowse{
		Settings: settings,
		Entries:  entries,
		// Facets describe the whole compendium so the dropdowns stay usable after
		// a filter narrows the result set.
		Facets:     domain.BuildCompendiumFacets(scoped),
		TotalCount: len(scoped),
		MatchCount: len(entries),
	}, nil
}

// compendiumEntryLess orders compendium entries. Weight and value sort on the same
// per-unit figures the card and the weight/value filters use (item.WeightHundredthsLB,
// item.BaseValueCP) rather than stack totals, so a stack's position in the list never
// contradicts the numbers shown on its own card. Every other key defers to the
// shared itemLess ordering used by /search.
func compendiumEntryLess(a, b domain.Item, sortBy string) bool {
	switch sortBy {
	case "weight":
		return a.WeightHundredthsLB < b.WeightHundredthsLB
	case "value":
		return a.BaseValueCP < b.BaseValueCP
	default:
		return itemLess(a, b, sortBy)
	}
}

// compendiumScopedItems returns every item held by the compendium: root items plus
// anything nested inside a compendium container. Vault-owned items are excluded.
func compendiumScopedItems(items []domain.Item) []domain.Item {
	scoped := make([]domain.Item, 0, len(items))
	for _, item := range items {
		if item.Location.OwnerVaultID != "" {
			continue
		}
		switch item.Location.Kind {
		case domain.LocationKindCompendiumRoot, domain.LocationKindContainer:
			scoped = append(scoped, item)
		}
	}
	return scoped
}

// parseOptionalHundredths parses a decimal bound into hundredths, returning nil for
// a blank input so the bound stays unset.
func parseOptionalHundredths(value, label string) (*int, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	parsed, err := domain.ParseWeightHundredths(value)
	if err != nil {
		return nil, fmt.Errorf("invalid %s: %w", label, err)
	}
	return &parsed, nil
}

// parseOptionalInt parses a whole-number bound, returning nil for a blank input.
func parseOptionalInt(value, label string) (*int, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, nil
	}
	parsed, err := strconv.Atoi(trimmed)
	if err != nil {
		return nil, fmt.Errorf("invalid %s %q", label, value)
	}
	return &parsed, nil
}

// parseTriState maps a yes/no select into an optional boolean filter.
func parseTriState(value string) *bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "yes", "true", "on":
		enabled := true
		return &enabled
	case "no", "false", "off":
		disabled := false
		return &disabled
	default:
		return nil
	}
}
