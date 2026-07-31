package domain

import (
	"fmt"
	"strconv"
	"strings"
)

// bulkKnownKeys is the authoritative set of attribute keys recognised by the
// bulk-import parser. Adding a key here together with a case in
// applyBulkAttribute is the only change needed to extend the format.
// BulkAttributeKeys() iterates this list for the format spec, and
// TestBulkFormatSpecCoversVocabulary ensures that the spec cannot drift.
var bulkKnownKeys = map[string]bool{
	// Weapon (category=weapon required).
	"damage": true, "dtype": true, "props": true, "class": true, "range": true,
	// Armor (category=armor required).
	"armor-class": true, "ac": true, "dex-mod": true, "str-req": true, "stealth-dis": true,
	// Tool (category=tool required).
	"tool-cat": true, "prof-notes": true,
	// Treasure (category=treasure required).
	"treasure-kind": true,
	// Vehicle (category=vehicle required).
	"vehicle-type": true,
	// Mount or Vehicle (category=mount or vehicle required).
	"speed": true, "carry-cap": true,
	// Container (category=container required).
	"max-weight": true, "max-volume": true, "max-liquid": true,
	// Per-row boolean overrides (no category restriction).
	"stackable": true, "container": true, "magical": true, "attunement": true, "equipped": true,
	// Per-row string overrides (no category restriction).
	"subcategory": true, "source": true, "location": true, "vault": true, "parent": true,
}

// BulkAttributeKeys returns all known attribute keys in a stable order so that
// BulkFormatSpec() and TestBulkFormatSpecCoversVocabulary can iterate them.
func BulkAttributeKeys() []string {
	return []string{
		// Weapon
		"damage", "dtype", "props", "class", "range",
		// Armor
		"armor-class", "ac", "dex-mod", "str-req", "stealth-dis",
		// Tool
		"tool-cat", "prof-notes",
		// Treasure
		"treasure-kind",
		// Vehicle
		"vehicle-type",
		// Mount/Vehicle
		"speed", "carry-cap",
		// Container
		"max-weight", "max-volume", "max-liquid",
		// Boolean overrides
		"stackable", "container", "magical", "attunement", "equipped",
		// String overrides
		"subcategory", "source", "location", "vault", "parent",
	}
}

// isKeyShape reports whether s matches the bulk key pattern: starts with a
// lowercase letter, followed by zero or more lowercase letters, digits,
// underscores, or hyphens. This is equivalent to ^[a-z][a-z0-9_-]*$.
func isKeyShape(s string) bool {
	if len(s) == 0 {
		return false
	}
	for i, c := range s {
		if i == 0 {
			if c < 'a' || c > 'z' {
				return false
			}
		} else {
			ok := (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' || c == '-'
			if !ok {
				return false
			}
		}
	}
	return true
}

// parseBulkBool interprets true/false/yes/no/1/0 case-insensitively.
func parseBulkBool(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "yes", "1":
		return true, nil
	case "false", "no", "0":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean %q, expected true/false/yes/no/1/0", value)
	}
}

// applyBulkAttribute tries to apply one pipe segment (possibly "key=value") to
// row. It returns keyShaped=true whenever the segment matches the key= pattern
// (regardless of whether the key is known). For unknown keys keyShaped is true
// and err is non-nil. For known keys err may be non-nil if the value is invalid
// or the row category is wrong. When keyShaped=false the caller should treat
// the segment as description text.
func applyBulkAttribute(seg string, row *BulkPreviewRow) (keyShaped bool, err error) {
	eqIdx := strings.IndexByte(seg, '=')
	if eqIdx < 0 {
		return false, nil
	}
	key := strings.TrimSpace(seg[:eqIdx])
	val := strings.TrimSpace(seg[eqIdx+1:])

	if !isKeyShape(key) {
		return false, nil
	}

	// Segment is key-shaped.
	if !bulkKnownKeys[key] {
		return true, fmt.Errorf("unknown attribute key %q", key)
	}

	switch key {

	// --- Weapon attributes (require category=weapon) ---

	case "damage":
		if row.Category != "weapon" {
			return true, fmt.Errorf("attribute %q requires category \"weapon\", got %q", key, row.Category)
		}
		if row.Details.Weapon == nil {
			row.Details.Weapon = &WeaponDetails{}
		}
		row.Details.Weapon.DamageDice = val

	case "dtype":
		if row.Category != "weapon" {
			return true, fmt.Errorf("attribute %q requires category \"weapon\", got %q", key, row.Category)
		}
		if row.Details.Weapon == nil {
			row.Details.Weapon = &WeaponDetails{}
		}
		row.Details.Weapon.DamageType = val

	case "props":
		if row.Category != "weapon" {
			return true, fmt.Errorf("attribute %q requires category \"weapon\", got %q", key, row.Category)
		}
		if row.Details.Weapon == nil {
			row.Details.Weapon = &WeaponDetails{}
		}
		parts := strings.Split(val, ",")
		normalized := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				normalized = append(normalized, p)
			}
		}
		row.Details.Weapon.Properties = NormalizeWeaponProperties(normalized)

	case "class":
		if row.Category != "weapon" {
			return true, fmt.Errorf("attribute %q requires category \"weapon\", got %q", key, row.Category)
		}
		if !IsValidWeaponCategory(val) {
			return true, fmt.Errorf("invalid weapon class %q; valid: %s", val, strings.Join(WeaponCategories(), ", "))
		}
		if row.Details.Weapon == nil {
			row.Details.Weapon = &WeaponDetails{}
		}
		row.Details.Weapon.WeaponClass = val

	case "range":
		if row.Category != "weapon" {
			return true, fmt.Errorf("attribute %q requires category \"weapon\", got %q", key, row.Category)
		}
		if row.Details.Weapon == nil {
			row.Details.Weapon = &WeaponDetails{}
		}
		if slashIdx := strings.IndexByte(val, '/'); slashIdx >= 0 {
			normal, nerr := strconv.Atoi(strings.TrimSpace(val[:slashIdx]))
			if nerr != nil {
				return true, fmt.Errorf("invalid range %q: normal range must be an integer", val)
			}
			long, lerr := strconv.Atoi(strings.TrimSpace(val[slashIdx+1:]))
			if lerr != nil {
				return true, fmt.Errorf("invalid range %q: long range must be an integer", val)
			}
			row.Details.Weapon.NormalRange = normal
			row.Details.Weapon.LongRange = long
		} else {
			normal, nerr := strconv.Atoi(val)
			if nerr != nil {
				return true, fmt.Errorf("invalid range %q: must be an integer or N/M format", val)
			}
			row.Details.Weapon.NormalRange = normal
		}

	// --- Armor attributes (require category=armor) ---

	case "armor-class", "ac":
		if row.Category != "armor" {
			return true, fmt.Errorf("attribute %q requires category \"armor\", got %q", key, row.Category)
		}
		n, nerr := strconv.Atoi(val)
		if nerr != nil {
			return true, fmt.Errorf("invalid %s value %q: must be an integer", key, val)
		}
		if row.Details.Armor == nil {
			row.Details.Armor = &ArmorDetails{}
		}
		row.Details.Armor.BaseAC = n

	case "dex-mod":
		if row.Category != "armor" {
			return true, fmt.Errorf("attribute %q requires category \"armor\", got %q", key, row.Category)
		}
		if row.Details.Armor == nil {
			row.Details.Armor = &ArmorDetails{}
		}
		row.Details.Armor.DexModifierBehavior = val

	case "str-req":
		if row.Category != "armor" {
			return true, fmt.Errorf("attribute %q requires category \"armor\", got %q", key, row.Category)
		}
		n, nerr := strconv.Atoi(val)
		if nerr != nil {
			return true, fmt.Errorf("invalid str-req value %q: must be an integer", val)
		}
		if row.Details.Armor == nil {
			row.Details.Armor = &ArmorDetails{}
		}
		row.Details.Armor.StrengthRequirement = n

	case "stealth-dis":
		if row.Category != "armor" {
			return true, fmt.Errorf("attribute %q requires category \"armor\", got %q", key, row.Category)
		}
		b, berr := parseBulkBool(val)
		if berr != nil {
			return true, berr
		}
		if row.Details.Armor == nil {
			row.Details.Armor = &ArmorDetails{}
		}
		row.Details.Armor.StealthDisadvantage = b

	// --- Tool attributes (require category=tool) ---

	case "tool-cat":
		if row.Category != "tool" {
			return true, fmt.Errorf("attribute %q requires category \"tool\", got %q", key, row.Category)
		}
		if row.Details.Tool == nil {
			row.Details.Tool = &ToolDetails{}
		}
		row.Details.Tool.ToolCategory = val

	case "prof-notes":
		if row.Category != "tool" {
			return true, fmt.Errorf("attribute %q requires category \"tool\", got %q", key, row.Category)
		}
		if row.Details.Tool == nil {
			row.Details.Tool = &ToolDetails{}
		}
		row.Details.Tool.ProficiencyNotes = val

	// --- Treasure attributes (require category=treasure) ---

	case "treasure-kind":
		if row.Category != "treasure" {
			return true, fmt.Errorf("attribute %q requires category \"treasure\", got %q", key, row.Category)
		}
		if row.Details.Treasure == nil {
			row.Details.Treasure = &TreasureDetails{}
		}
		row.Details.Treasure.TreasureKind = val

	// --- Vehicle attributes (require category=vehicle) ---

	case "vehicle-type":
		if row.Category != "vehicle" {
			return true, fmt.Errorf("attribute %q requires category \"vehicle\", got %q", key, row.Category)
		}
		if row.Details.Vehicle == nil {
			row.Details.Vehicle = &VehicleDetails{}
		}
		row.Details.Vehicle.VehicleType = val

	// --- Mount/Vehicle shared attributes ---

	case "speed":
		switch row.Category {
		case "mount":
			n, nerr := strconv.Atoi(val)
			if nerr != nil {
				return true, fmt.Errorf("invalid speed value %q: must be an integer", val)
			}
			if row.Details.Mount == nil {
				row.Details.Mount = &MountDetails{}
			}
			row.Details.Mount.MovementSpeed = n
		case "vehicle":
			n, nerr := strconv.Atoi(val)
			if nerr != nil {
				return true, fmt.Errorf("invalid speed value %q: must be an integer", val)
			}
			if row.Details.Vehicle == nil {
				row.Details.Vehicle = &VehicleDetails{}
			}
			row.Details.Vehicle.MovementSpeed = n
		default:
			return true, fmt.Errorf("attribute %q requires category \"mount\" or \"vehicle\", got %q", key, row.Category)
		}

	case "carry-cap":
		switch row.Category {
		case "mount":
			n, nerr := ParseWeightHundredths(val)
			if nerr != nil {
				return true, fmt.Errorf("invalid carry-cap: %w", nerr)
			}
			if row.Details.Mount == nil {
				row.Details.Mount = &MountDetails{}
			}
			row.Details.Mount.CarryingCapacityHundredthsLB = n
		case "vehicle":
			n, nerr := ParseWeightHundredths(val)
			if nerr != nil {
				return true, fmt.Errorf("invalid carry-cap: %w", nerr)
			}
			if row.Details.Vehicle == nil {
				row.Details.Vehicle = &VehicleDetails{}
			}
			row.Details.Vehicle.CarryingCapacityHundredthsLB = n
		default:
			return true, fmt.Errorf("attribute %q requires category \"mount\" or \"vehicle\", got %q", key, row.Category)
		}

	// --- Container attributes (require category=container) ---

	case "max-weight":
		if row.Category != "container" {
			return true, fmt.Errorf("attribute %q requires category \"container\", got %q", key, row.Category)
		}
		n, nerr := ParseWeightHundredths(val)
		if nerr != nil {
			return true, fmt.Errorf("invalid max-weight: %w", nerr)
		}
		if row.Details.Container == nil {
			row.Details.Container = &ContainerDetails{}
		}
		row.Details.Container.MaxWeightHundredthsLB = n

	case "max-volume":
		if row.Category != "container" {
			return true, fmt.Errorf("attribute %q requires category \"container\", got %q", key, row.Category)
		}
		n, nerr := strconv.Atoi(val)
		if nerr != nil {
			return true, fmt.Errorf("invalid max-volume value %q: must be an integer", val)
		}
		if row.Details.Container == nil {
			row.Details.Container = &ContainerDetails{}
		}
		row.Details.Container.MaxVolumeCubicInches = n

	case "max-liquid":
		if row.Category != "container" {
			return true, fmt.Errorf("attribute %q requires category \"container\", got %q", key, row.Category)
		}
		n, nerr := strconv.Atoi(val)
		if nerr != nil {
			return true, fmt.Errorf("invalid max-liquid value %q: must be an integer", val)
		}
		if row.Details.Container == nil {
			row.Details.Container = &ContainerDetails{}
		}
		row.Details.Container.MaxLiquidOunces = n

	// --- Per-row boolean overrides (no category restriction) ---

	case "stackable":
		b, berr := parseBulkBool(val)
		if berr != nil {
			return true, berr
		}
		row.IsStackable = &b

	case "container":
		b, berr := parseBulkBool(val)
		if berr != nil {
			return true, berr
		}
		row.IsContainer = &b

	case "magical":
		b, berr := parseBulkBool(val)
		if berr != nil {
			return true, berr
		}
		row.IsMagical = &b

	case "attunement":
		b, berr := parseBulkBool(val)
		if berr != nil {
			return true, berr
		}
		row.RequiresAttunement = &b

	case "equipped":
		b, berr := parseBulkBool(val)
		if berr != nil {
			return true, berr
		}
		row.IsEquipped = &b

	// --- Per-row string overrides (no category restriction) ---

	case "subcategory":
		row.Subcategory = val

	case "source":
		sk, ok := TryParseSourceKind(val)
		if !ok {
			return true, fmt.Errorf("invalid source kind %q; valid: manual, custom, imported", val)
		}
		row.SourceKind = sk

	case "location":
		lk := LocationKind(strings.ToLower(val))
		if !lk.Valid() {
			return true, fmt.Errorf("invalid location kind %q; valid: vault, container, compendium", val)
		}
		row.LocationKind = lk

	case "vault":
		row.VaultName = val

	case "parent":
		row.ContainerName = val
	}

	return true, nil
}
