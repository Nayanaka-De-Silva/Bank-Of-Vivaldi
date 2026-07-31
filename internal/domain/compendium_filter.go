package domain

import (
	"sort"
	"strings"
)

// CompendiumCriteria is a parsed, transport-agnostic description of a compendium
// browse filter. Nil pointer fields mean "unbounded" or "don't care", so a zero
// value matches every item.
type CompendiumCriteria struct {
	Query    string
	Category string
	Rarity   string

	MinWeightHundredthsLB *int
	MaxWeightHundredthsLB *int
	MinValueCP            *int
	MaxValueCP            *int

	IsMagical          *bool
	RequiresAttunement *bool

	Armor     ArmorCriteria
	Weapon    WeaponCriteria
	Container ContainerCriteria
	Tool      ToolCriteria
	Mount     MountCriteria
	Vehicle   VehicleCriteria
	Treasure  TreasureCriteria
}

// ArmorCriteria narrows items by their armor metadata.
type ArmorCriteria struct {
	Category            string
	DexBehavior         string
	MinBaseAC           *int
	MaxBaseAC           *int
	StealthDisadvantage *bool
}

// WeaponCriteria narrows items by their weapon metadata.
type WeaponCriteria struct {
	Category   string
	DamageType string
	Property   string
}

// ContainerCriteria narrows items by their carrying capacity.
type ContainerCriteria struct {
	MinCapacityHundredthsLB *int
	MaxCapacityHundredthsLB *int
}

// ToolCriteria narrows items by their tool metadata.
type ToolCriteria struct {
	Category string
}

// MountCriteria narrows items by their mount metadata.
type MountCriteria struct {
	Type     string
	MinSpeed *int
	MaxSpeed *int
}

// VehicleCriteria narrows items by their vehicle metadata.
type VehicleCriteria struct {
	Type     string
	MinSpeed *int
	MaxSpeed *int
}

// TreasureCriteria narrows items by their treasure metadata.
type TreasureCriteria struct {
	Kind string
}

// Matches reports whether an item satisfies every active part of the criteria.
// Metadata criteria are only consulted for the selected category, so filters left
// over from a previous category selection never affect the result.
func (c CompendiumCriteria) Matches(item Item) bool {
	if !matchesText(item.Category, c.Category) {
		return false
	}
	if !matchesText(string(item.Rarity), c.Rarity) {
		return false
	}
	if !matchesQuery(item, c.Query) {
		return false
	}
	if !withinBounds(item.WeightHundredthsLB, c.MinWeightHundredthsLB, c.MaxWeightHundredthsLB) {
		return false
	}
	if !withinBounds(item.BaseValueCP, c.MinValueCP, c.MaxValueCP) {
		return false
	}
	if !matchesFlag(item.IsMagical, c.IsMagical) {
		return false
	}
	if !matchesFlag(item.RequiresAttunement, c.RequiresAttunement) {
		return false
	}

	switch strings.ToLower(strings.TrimSpace(c.Category)) {
	case "armor":
		return c.Armor.matches(item)
	case "weapon":
		return c.Weapon.matches(item)
	case "container":
		return c.Container.matches(item)
	case "tool":
		return c.Tool.matches(item)
	case "mount":
		return c.Mount.matches(item)
	case "vehicle":
		return c.Vehicle.matches(item)
	case "treasure":
		return c.Treasure.matches(item)
	default:
		return true
	}
}

func (a ArmorCriteria) isZero() bool {
	return a.Category == "" && a.DexBehavior == "" && a.MinBaseAC == nil && a.MaxBaseAC == nil && a.StealthDisadvantage == nil
}

func (a ArmorCriteria) matches(item Item) bool {
	if a.isZero() {
		return true
	}
	details := item.Details.Armor
	if details == nil {
		return false
	}
	if !matchesText(details.ArmorCategory, a.Category) {
		return false
	}
	if !matchesText(details.DexModifierBehavior, a.DexBehavior) {
		return false
	}
	if !withinBounds(details.BaseAC, a.MinBaseAC, a.MaxBaseAC) {
		return false
	}
	return matchesFlag(details.StealthDisadvantage, a.StealthDisadvantage)
}

func (w WeaponCriteria) isZero() bool {
	return w.Category == "" && w.DamageType == "" && w.Property == ""
}

func (w WeaponCriteria) matches(item Item) bool {
	if w.isZero() {
		return true
	}
	details := item.Details.Weapon
	if details == nil {
		return false
	}
	if !matchesText(details.WeaponClass, w.Category) {
		return false
	}
	if !matchesText(details.DamageType, w.DamageType) {
		return false
	}
	if strings.TrimSpace(w.Property) == "" {
		return true
	}
	for _, property := range details.Properties {
		// EqualWeaponProperty tolerates case/separator variants (e.g. "Two Handed" == "two-handed").
		if EqualWeaponProperty(property, w.Property) {
			return true
		}
	}
	return false
}

func (c ContainerCriteria) matches(item Item) bool {
	if c.MinCapacityHundredthsLB == nil && c.MaxCapacityHundredthsLB == nil {
		return true
	}
	if item.Details.Container == nil {
		return false
	}
	return withinBounds(item.Details.Container.MaxWeightHundredthsLB, c.MinCapacityHundredthsLB, c.MaxCapacityHundredthsLB)
}

func (t ToolCriteria) matches(item Item) bool {
	if strings.TrimSpace(t.Category) == "" {
		return true
	}
	if item.Details.Tool == nil {
		return false
	}
	return matchesText(item.Details.Tool.ToolCategory, t.Category)
}

func (m MountCriteria) matches(item Item) bool {
	if m.Type == "" && m.MinSpeed == nil && m.MaxSpeed == nil {
		return true
	}
	details := item.Details.Mount
	if details == nil {
		return false
	}
	if !matchesText(details.MountType, m.Type) {
		return false
	}
	return withinBounds(details.MovementSpeed, m.MinSpeed, m.MaxSpeed)
}

func (v VehicleCriteria) matches(item Item) bool {
	if v.Type == "" && v.MinSpeed == nil && v.MaxSpeed == nil {
		return true
	}
	details := item.Details.Vehicle
	if details == nil {
		return false
	}
	if !matchesText(details.VehicleType, v.Type) {
		return false
	}
	return withinBounds(details.MovementSpeed, v.MinSpeed, v.MaxSpeed)
}

func (t TreasureCriteria) matches(item Item) bool {
	if strings.TrimSpace(t.Kind) == "" {
		return true
	}
	if item.Details.Treasure == nil {
		return false
	}
	return matchesText(item.Details.Treasure.TreasureKind, t.Kind)
}

// matchesText compares case-insensitively; an empty wanted value matches anything.
func matchesText(actual, wanted string) bool {
	wanted = strings.TrimSpace(wanted)
	if wanted == "" {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(actual), wanted)
}

// withinBounds checks an inclusive range where nil bounds are unbounded.
func withinBounds(value int, min, max *int) bool {
	if min != nil && value < *min {
		return false
	}
	if max != nil && value > *max {
		return false
	}
	return true
}

// matchesFlag compares a boolean field against a tri-state filter.
func matchesFlag(actual bool, wanted *bool) bool {
	return wanted == nil || actual == *wanted
}

func matchesQuery(item Item, query string) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return true
	}
	haystack := strings.ToLower(strings.Join([]string{
		item.Name,
		item.Description,
		item.Category,
		item.Subcategory,
		string(item.Rarity),
	}, " "))
	return strings.Contains(haystack, query)
}

// CompendiumFacets holds the distinct metadata values present in a set of items so
// the conditional filter dropdowns can be built from real data rather than a
// hardcoded list.
type CompendiumFacets struct {
	ArmorCategories   []string
	ArmorDexBehaviors []string
	WeaponDamageTypes []string
	WeaponProperties  []string
	ToolCategories    []string
	MountTypes        []string
	VehicleTypes      []string
	TreasureKinds     []string
}

// BuildCompendiumFacets collects the distinct metadata values across the items,
// deduplicated case-insensitively and sorted alphabetically.
func BuildCompendiumFacets(items []Item) CompendiumFacets {
	armorCategories := newFacetSet()
	armorDexBehaviors := newFacetSet()
	weaponDamageTypes := newFacetSet()
	weaponProperties := newFacetSet()
	toolCategories := newFacetSet()
	mountTypes := newFacetSet()
	vehicleTypes := newFacetSet()
	treasureKinds := newFacetSet()

	for _, item := range items {
		if details := item.Details.Armor; details != nil {
			armorCategories.add(details.ArmorCategory)
			armorDexBehaviors.add(details.DexModifierBehavior)
		}
		if details := item.Details.Weapon; details != nil {
			weaponDamageTypes.add(details.DamageType)
			for _, property := range details.Properties {
				weaponProperties.add(property)
			}
		}
		if details := item.Details.Tool; details != nil {
			toolCategories.add(details.ToolCategory)
		}
		if details := item.Details.Mount; details != nil {
			mountTypes.add(details.MountType)
		}
		if details := item.Details.Vehicle; details != nil {
			vehicleTypes.add(details.VehicleType)
		}
		if details := item.Details.Treasure; details != nil {
			treasureKinds.add(details.TreasureKind)
		}
	}

	return CompendiumFacets{
		ArmorCategories:   armorCategories.values(),
		ArmorDexBehaviors: armorDexBehaviors.values(),
		WeaponDamageTypes: weaponDamageTypes.values(),
		WeaponProperties:  weaponProperties.values(),
		ToolCategories:    toolCategories.values(),
		MountTypes:        mountTypes.values(),
		VehicleTypes:      vehicleTypes.values(),
		TreasureKinds:     treasureKinds.values(),
	}
}

// facetSet collects distinct display values keyed by their lowercase form.
type facetSet map[string]string

func newFacetSet() facetSet {
	return facetSet{}
}

func (s facetSet) add(value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	key := strings.ToLower(value)
	if _, exists := s[key]; !exists {
		s[key] = value
	}
}

func (s facetSet) values() []string {
	keys := make([]string, 0, len(s))
	for key := range s {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	values := make([]string, 0, len(keys))
	for _, key := range keys {
		values = append(values, s[key])
	}
	return values
}
