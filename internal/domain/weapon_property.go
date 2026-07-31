package domain

import "strings"

// WeaponProperty describes one PHB weapon property with its canonical key, display label, and tooltip description.
type WeaponProperty struct {
	Key, Label, Description string
}

// WeaponProperties returns the ten D&D 5e PHB weapon properties in book order.
func WeaponProperties() []WeaponProperty {
	return []WeaponProperty{
		{
			Key:         "ammunition",
			Label:       "Ammunition",
			Description: "Ranged attacks require ammo; one piece is expended per attack. Used in melee it counts as an improvised weapon. You can recover about half of expended ammunition after a fight.",
		},
		{
			Key:         "finesse",
			Label:       "Finesse",
			Description: "Use either Strength or Dexterity for the attack and damage rolls, but you must use the same modifier for both.",
		},
		{
			Key:         "heavy",
			Label:       "Heavy",
			Description: "Small creatures have disadvantage on attack rolls with heavy weapons.",
		},
		{
			Key:         "light",
			Label:       "Light",
			Description: "A light weapon is small and easy to handle, making it ideal for two-weapon fighting.",
		},
		{
			Key:         "loading",
			Label:       "Loading",
			Description: "Only one piece of ammunition can be fired per action, bonus action, or reaction, regardless of the number of extra attacks.",
		},
		{
			Key:         "reach",
			Label:       "Reach",
			Description: "This weapon adds 5 feet to your reach when you attack with it.",
		},
		{
			Key:         "special",
			Label:       "Special",
			Description: "This weapon has unusual rules described in its own entry.",
		},
		{
			Key:         "thrown",
			Label:       "Thrown",
			Description: "You can throw this weapon to make a ranged attack. If the weapon is a melee weapon, you use the same modifier for the attack and damage rolls as you would for a melee attack.",
		},
		{
			Key:         "two-handed",
			Label:       "Two-Handed",
			Description: "This weapon requires two hands to use.",
		},
		{
			Key:         "versatile",
			Label:       "Versatile",
			Description: "This weapon can be used one- or two-handed; a larger damage die listed in parentheses applies when used with two hands.",
		},
	}
}

// normalizeWeaponPropertyKey trims and slugifies a property value for lookup.
// Returns "" for empty input, guarding against Slugify's "item" fallback for blank input.
func normalizeWeaponPropertyKey(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return Slugify(value)
}

// LookupWeaponProperty finds a canonical property by its key or any case/separator variant.
func LookupWeaponProperty(value string) (WeaponProperty, bool) {
	key := normalizeWeaponPropertyKey(value)
	if key == "" {
		return WeaponProperty{}, false
	}
	for _, prop := range WeaponProperties() {
		if prop.Key == key {
			return prop, true
		}
	}
	return WeaponProperty{}, false
}

// IsValidWeaponProperty reports whether value refers to one of the ten canonical properties.
func IsValidWeaponProperty(value string) bool {
	_, ok := LookupWeaponProperty(value)
	return ok
}

// CanonicalWeaponProperty returns the canonical key for a known property,
// or the trimmed original value for an unknown one (never silently alters free-text data).
func CanonicalWeaponProperty(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if prop, ok := LookupWeaponProperty(trimmed); ok {
		return prop.Key
	}
	return trimmed
}

// NormalizeWeaponProperties canonicalizes each value, drops blanks, deduplicates, and preserves order.
func NormalizeWeaponProperties(values []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, v := range values {
		c := CanonicalWeaponProperty(v)
		if c == "" || seen[c] {
			continue
		}
		seen[c] = true
		result = append(result, c)
	}
	return result
}

// ResolveWeaponProperties looks up each value; unknown values get an empty Description and a humanized Label.
func ResolveWeaponProperties(values []string) []WeaponProperty {
	props := make([]WeaponProperty, 0, len(values))
	for _, v := range values {
		if prop, ok := LookupWeaponProperty(v); ok {
			props = append(props, prop)
		} else {
			props = append(props, WeaponProperty{
				Key:         v,
				Label:       HumanizeLabel(v),
				Description: "",
			})
		}
	}
	return props
}

// EqualWeaponProperty reports whether a and b refer to the same property, case- and separator-tolerant.
func EqualWeaponProperty(a, b string) bool {
	ka := normalizeWeaponPropertyKey(a)
	kb := normalizeWeaponPropertyKey(b)
	return ka != "" && kb != "" && ka == kb
}

// WeaponPropertyChoice is one form option with selection and canonical-status metadata.
type WeaponPropertyChoice struct {
	WeaponProperty
	Selected, Known bool
}

// WeaponPropertyChoices returns the ten canonical choices (Known=true) with Selected set per
// selected, followed by any value in selected that isn't canonical as Known=false, Selected=true,
// so legacy free-text properties round-trip through the form without being silently dropped.
func WeaponPropertyChoices(selected []string) []WeaponPropertyChoice {
	// Normalize selected to canonical keys for quick lookup.
	selectedSet := make(map[string]bool)
	for _, v := range selected {
		if k := CanonicalWeaponProperty(v); k != "" {
			selectedSet[k] = true
		}
	}

	canonical := WeaponProperties()
	canonicalKeys := make(map[string]bool, len(canonical))
	choices := make([]WeaponPropertyChoice, 0, len(canonical)+len(selected))
	for _, prop := range canonical {
		canonicalKeys[prop.Key] = true
		choices = append(choices, WeaponPropertyChoice{
			WeaponProperty: prop,
			Selected:       selectedSet[prop.Key],
			Known:          true,
		})
	}

	// Append non-canonical selected values as legacy entries.
	seenLegacy := make(map[string]bool)
	for _, v := range selected {
		k := CanonicalWeaponProperty(v)
		if k == "" || canonicalKeys[k] || seenLegacy[k] {
			continue
		}
		seenLegacy[k] = true
		choices = append(choices, WeaponPropertyChoice{
			WeaponProperty: WeaponProperty{
				Key:         k,
				Label:       HumanizeLabel(k),
				Description: "",
			},
			Selected: true,
			Known:    false,
		})
	}

	return choices
}

// WeaponPropertyOptions returns the ten canonical keys followed by any non-canonical value in stored.
// Used to build the compendium filter dropdown so legacy data stays filterable even in an empty compendium.
func WeaponPropertyOptions(stored []string) []string {
	canonical := WeaponProperties()
	canonicalKeys := make(map[string]bool, len(canonical))
	options := make([]string, 0, len(canonical)+len(stored))
	for _, prop := range canonical {
		canonicalKeys[prop.Key] = true
		options = append(options, prop.Key)
	}

	seenExtra := make(map[string]bool)
	for _, v := range stored {
		k := CanonicalWeaponProperty(v)
		if k == "" || canonicalKeys[k] || seenExtra[k] {
			continue
		}
		seenExtra[k] = true
		options = append(options, k)
	}

	return options
}
