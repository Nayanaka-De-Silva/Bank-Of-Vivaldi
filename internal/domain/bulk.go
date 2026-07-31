package domain

import (
	"fmt"
	"strconv"
	"strings"
)

type BulkDefaults struct {
	Category           string
	Rarity             Rarity
	WeightHundredthsLB int
	BaseValueCP        int
}

// BulkPreviewRow holds one parsed line from a bulk-import paste. Every new field
// carries a json tag so it survives the base64+JSON round-trip through EncodeBulkRows.
// Pointer bools distinguish "row explicitly set false" from "row was silent" when
// the batch default is true.
type BulkPreviewRow struct {
	LineNumber          int          `json:"line_number"`
	Raw                 string       `json:"raw"`
	Name                string       `json:"name"`
	Quantity            int          `json:"quantity"`
	Category            string       `json:"category"`
	Rarity              Rarity       `json:"rarity"`
	WeightHundredthsLB  int          `json:"weight_hundredths_lb"`
	BaseValueCP         int          `json:"base_value_cp"`
	Errors              []string     `json:"errors,omitempty"`
	Description         string       `json:"description,omitempty"`
	Subcategory         string       `json:"subcategory,omitempty"`
	Details             ItemDetails  `json:"details"`
	SourceKind          SourceKind   `json:"source_kind,omitempty"`
	LocationKind        LocationKind `json:"location_kind,omitempty"`
	VaultName           string       `json:"vault_name,omitempty"`
	ContainerName       string       `json:"container_name,omitempty"`
	ResolvedVaultID     string       `json:"resolved_vault_id,omitempty"`
	ResolvedContainerID string       `json:"resolved_container_id,omitempty"`
	IsStackable         *bool        `json:"is_stackable,omitempty"`
	IsContainer         *bool        `json:"is_container,omitempty"`
	IsMagical           *bool        `json:"is_magical,omitempty"`
	RequiresAttunement  *bool        `json:"requires_attunement,omitempty"`
	IsEquipped          *bool        `json:"is_equipped,omitempty"`
}

type BulkPreview struct {
	Rows []BulkPreviewRow `json:"rows"`
}

func (p BulkPreview) Valid() bool {
	for _, row := range p.Rows {
		if len(row.Errors) > 0 {
			return false
		}
	}
	return true
}

// ParseBulkItemInput parses a multi-line bulk paste into a BulkPreview.
// Each line uses the format:
//
//	[qty]Name | category | rarity | weight_lb | value | description | key=value ...
//
// Category and rarity are validated strictly; an unknown value is a row error.
// Value accepts a denomination suffix (2gp, 50sp, 1,000gp) or bare copper.
// Segments past index 4 are classified individually: known key=value pairs are
// applied as attributes, unknown key-shaped pairs are errors, and everything
// else becomes description text (stray pipes are rejoined with " | ").
func ParseBulkItemInput(text string, defaults BulkDefaults) BulkPreview {
	lines := strings.Split(text, "\n")
	rows := make([]BulkPreviewRow, 0, len(lines))

	// Beware the Pinkertons of WOTC.
	for index, line := range lines {
		raw := strings.TrimSpace(line)
		if raw == "" {
			continue
		}

		row := BulkPreviewRow{
			LineNumber:         index + 1,
			Raw:                raw,
			Quantity:           1,
			Category:           defaults.Category,
			Rarity:             defaults.Rarity,
			WeightHundredthsLB: defaults.WeightHundredthsLB,
			BaseValueCP:        defaults.BaseValueCP,
		}

		parts := strings.Split(raw, "|")
		for idx := range parts {
			parts[idx] = strings.TrimSpace(parts[idx])
		}

		row.Name, row.Quantity = parseBulkName(parts[0])
		if row.Name == "" {
			row.Errors = append(row.Errors, "name is required")
		}

		if len(parts) > 1 && parts[1] != "" {
			cat := strings.ToLower(parts[1])
			if !IsValidCategory(cat) {
				row.Errors = append(row.Errors, fmt.Sprintf("unknown category %q", parts[1]))
			} else {
				row.Category = cat
			}
		}
		if len(parts) > 2 && parts[2] != "" {
			rarity, ok := TryParseRarity(parts[2])
			if !ok {
				row.Errors = append(row.Errors, fmt.Sprintf("unknown rarity %q", parts[2]))
			} else {
				row.Rarity = rarity
			}
		}
		if len(parts) > 3 && parts[3] != "" {
			weight, err := ParseWeightHundredths(parts[3])
			if err != nil {
				row.Errors = append(row.Errors, err.Error())
			} else {
				row.WeightHundredthsLB = weight
			}
		}
		if len(parts) > 4 && parts[4] != "" {
			value, err := ParseDenominatedCopper(parts[4])
			if err != nil {
				row.Errors = append(row.Errors, fmt.Sprintf("invalid value %q", parts[4]))
			} else {
				row.BaseValueCP = value
			}
		}

		// Classify segments 5+ as description text or key=value attributes.
		var descParts []string
		for i := 5; i < len(parts); i++ {
			seg := parts[i]
			if seg == "" {
				continue
			}
			keyShaped, err := applyBulkAttribute(seg, &row)
			if err != nil {
				row.Errors = append(row.Errors, err.Error())
			} else if !keyShaped {
				descParts = append(descParts, seg)
			}
		}
		if len(descParts) > 0 {
			row.Description = strings.Join(descParts, " | ")
		}

		if row.Quantity < 1 {
			row.Errors = append(row.Errors, "quantity must be at least 1")
		}

		rows = append(rows, row)
	}

	return BulkPreview{Rows: rows}
}

func parseBulkName(value string) (string, int) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", 1
	}

	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		return "", 1
	}

	if quantity, err := strconv.Atoi(strings.TrimSuffix(strings.ToLower(fields[0]), "x")); err == nil && len(fields) > 1 {
		return strings.Join(fields[1:], " "), quantity
	}

	lastField := fields[len(fields)-1]
	if quantity, err := strconv.Atoi(strings.TrimPrefix(strings.ToLower(lastField), "x")); err == nil && len(fields) > 1 {
		return strings.Join(fields[:len(fields)-1], " "), quantity
	}

	return trimmed, 1
}

// BulkFormatSpec generates a self-contained grammar spec from the live
// vocabulary — Categories(), Rarities(), WeaponProperties(), WeaponCategories(),
// and BulkAttributeKeys() — so the documented format can never drift from the
// parser. The text is designed to be pasted verbatim to an AI agent.
func BulkFormatSpec() string {
	var b strings.Builder

	b.WriteString("BULK ITEM IMPORT FORMAT\n")
	b.WriteString("=======================\n\n")

	b.WriteString("Grammar (one item per line):\n")
	b.WriteString("  [qty]Name | category | rarity | weight_lb | value | description | key=value ...\n\n")

	b.WriteString("Positional fields (0–4). All are optional except Name.\n")
	b.WriteString("  qty         optional integer prefix or suffix: \"2x Rope\" or \"Rope 2x\"\n")
	b.WriteString("  category    must be one of the valid categories listed below (strict)\n")
	b.WriteString("  rarity      must be one of the valid rarities listed below (strict)\n")
	b.WriteString("  weight_lb   decimal pounds, e.g. 1, 0.5, 2.25\n")
	b.WriteString("  value       denominated coin value (2gp, 1,000gp, 50sp, 100cp) or bare integer = copper\n\n")

	b.WriteString("Segments 5+ are unordered and pipe-separated. Each is classified as:\n")
	b.WriteString("  known key=value attribute  -> applied to the item\n")
	b.WriteString("  unknown key-shaped segment -> ERROR (key matches ^[a-z][a-z0-9_-]*=)\n")
	b.WriteString("  anything else              -> description text (stray pipes rejoin with \" | \")\n\n")

	b.WriteString("Valid categories:\n")
	for _, cat := range Categories() {
		b.WriteString("  " + cat + "\n")
	}
	b.WriteString("\n")

	b.WriteString("Valid rarities:\n")
	for _, r := range Rarities() {
		b.WriteString("  " + r + "\n")
	}
	b.WriteString("\n")

	b.WriteString("Valid weapon classes (for class= attribute):\n")
	for _, wc := range WeaponCategories() {
		b.WriteString("  " + wc + "\n")
	}
	b.WriteString("\n")

	b.WriteString("Valid weapon properties (for props= attribute, comma-separated):\n")
	for _, wp := range WeaponProperties() {
		b.WriteString("  " + wp.Key + "\n")
	}
	b.WriteString("\n")

	b.WriteString("Attribute keys by category:\n")
	b.WriteString("  Weapon  (category=weapon required):           damage dtype props class range versatile\n")
	b.WriteString("  Armor   (category=armor required):            armor-class ac dex-mod str-req stealth-dis\n")
	b.WriteString("  Tool    (category=tool required):             tool-cat prof-notes\n")
	b.WriteString("  Treasure (category=treasure required):        treasure-kind\n")
	b.WriteString("  Vehicle (category=vehicle required):          vehicle-type\n")
	b.WriteString("  Mount/Vehicle (category=mount or vehicle):    speed carry-cap\n")
	b.WriteString("  Container (category=container required):      max-weight max-volume max-liquid\n")
	b.WriteString("  Per-row overrides (no category restriction):\n")
	b.WriteString("    stackable container magical attunement equipped\n")
	b.WriteString("    subcategory source location vault parent\n\n")

	b.WriteString("NAMING COLLISIONS (common points of confusion):\n")
	b.WriteString("  class=      -> weapon subcategory (e.g. simple-melee); NOT for armor\n")
	b.WriteString("  armor-class=-> armor base AC; use this for armor, NOT class=\n")
	b.WriteString("  container=  -> IsContainer boolean flag (true/false)\n")
	b.WriteString("  parent=     -> destination container name (the item to place this inside)\n\n")

	b.WriteString("Boolean values: true, false, yes, no, 1, 0 (case-insensitive)\n\n")

	b.WriteString("All attribute keys (canonical reference):\n")
	for _, key := range BulkAttributeKeys() {
		b.WriteString("  " + key + "\n")
	}
	b.WriteString("\n")

	b.WriteString("Examples:\n")
	b.WriteString("  Rope | equipment | mundane | 10 | 1gp\n")
	b.WriteString("  2x Torch | equipment | mundane | 1 | 1cp | A tallow torch, burns for 1 hour.\n")
	b.WriteString("  Dagger | weapon | mundane | 1 | 2gp | A simple blade. | damage=1d4 | dtype=piercing | props=finesse,light,thrown | class=simple-melee | range=20/60\n")
	b.WriteString("  Chain Mail | armor | mundane | 55 | 75gp | Heavy steel links. | armor-class=16 | str-req=13 | stealth-dis=true\n")
	b.WriteString("  Backpack | container | mundane | 5 | 2gp | Holds 30 lb of gear. | max-weight=30\n")
	b.WriteString("  +1 Longsword | weapon | uncommon | 3 | 1,500gp | A magic blade. | magical=true | damage=1d8 | dtype=slashing | vault=Lyra\n")

	return b.String()
}
