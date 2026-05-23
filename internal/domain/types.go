package domain

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type EncumbranceMode string

const (
	EncumbranceModeStandard EncumbranceMode = "standard"
	EncumbranceModeOptional EncumbranceMode = "optional"
)

func (m EncumbranceMode) Valid() bool {
	return m == EncumbranceModeStandard || m == EncumbranceModeOptional
}

func ParseEncumbranceMode(value string) EncumbranceMode {
	mode := EncumbranceMode(strings.ToLower(strings.TrimSpace(value)))
	if !mode.Valid() {
		return EncumbranceModeStandard
	}
	return mode
}

type EncumbranceState string

const (
	EncumbranceStateNormal           EncumbranceState = "normal"
	EncumbranceStateEncumbered       EncumbranceState = "encumbered"
	EncumbranceStateHeavilyEncumbered EncumbranceState = "heavily-encumbered"
	EncumbranceStateOverCapacity     EncumbranceState = "over-capacity"
)

type LocationKind string

const (
	LocationKindCompendiumRoot LocationKind = "compendium"
	LocationKindVaultRoot      LocationKind = "vault"
	LocationKindContainer      LocationKind = "container"
)

func (k LocationKind) Valid() bool {
	return k == LocationKindCompendiumRoot || k == LocationKindVaultRoot || k == LocationKindContainer
}

type Rarity string

const (
	RarityMundane   Rarity = "mundane"
	RarityUnknown   Rarity = "unknown"
	RarityCommon    Rarity = "common"
	RarityUncommon  Rarity = "uncommon"
	RarityRare      Rarity = "rare"
	RarityVeryRare  Rarity = "very-rare"
	RarityLegendary Rarity = "legendary"
	RarityArtifact  Rarity = "artifact"
)

func ParseRarity(value string) Rarity {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch Rarity(normalized) {
	case RarityMundane, RarityUnknown, RarityCommon, RarityUncommon, RarityRare, RarityVeryRare, RarityLegendary, RarityArtifact:
		return Rarity(normalized)
	default:
		return RarityMundane
	}
}

type SourceKind string

const (
	SourceKindManual   SourceKind = "manual"
	SourceKindCustom   SourceKind = "custom"
	SourceKindImported SourceKind = "imported"
)

func ParseSourceKind(value string) SourceKind {
	normalized := SourceKind(strings.ToLower(strings.TrimSpace(value)))
	switch normalized {
	case SourceKindManual, SourceKindCustom, SourceKindImported:
		return normalized
	default:
		return SourceKindManual
	}
}

type AppSettings struct {
	DefaultEncumbranceMode EncumbranceMode
}

type Purse struct {
	CP int `json:"cp"`
	SP int `json:"sp"`
	EP int `json:"ep"`
	GP int `json:"gp"`
	PP int `json:"pp"`
}

func (p Purse) TotalCoins() int {
	return p.CP + p.SP + p.EP + p.GP + p.PP
}

type Vault struct {
	ID              string
	CharacterName   string
	StrengthScore   int
	CarryModifierLB int
	EncumbranceMode EncumbranceMode
	Notes           string
	Archived        bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
	Purse           Purse
}

func (v Vault) CarryCapacityHundredthsLB() int {
	return ((v.StrengthScore * 15) + v.CarryModifierLB) * 100
}

func (v Vault) PushDragLiftHundredthsLB() int {
	return ((v.StrengthScore * 30) + v.CarryModifierLB) * 100
}

type ItemLocation struct {
	Kind                  LocationKind `json:"kind"`
	OwnerVaultID          string       `json:"owner_vault_id,omitempty"`
	ParentContainerItemID string       `json:"parent_container_item_id,omitempty"`
}

type ContainerDetails struct {
	MaxWeightHundredthsLB int `json:"max_weight_hundredths_lb"`
	MaxVolumeCubicInches  int `json:"max_volume_cubic_inches,omitempty"`
	MaxLiquidOunces       int `json:"max_liquid_ounces,omitempty"`
}

type ArmorDetails struct {
	ArmorCategory       string `json:"armor_category,omitempty"`
	BaseAC              int    `json:"base_ac,omitempty"`
	DexModifierBehavior string `json:"dex_modifier_behavior,omitempty"`
	StrengthRequirement int    `json:"strength_requirement,omitempty"`
	StealthDisadvantage bool   `json:"stealth_disadvantage,omitempty"`
}

type WeaponDetails struct {
	WeaponClass string   `json:"weapon_class,omitempty"`
	DamageDice  string   `json:"damage_dice,omitempty"`
	DamageType  string   `json:"damage_type,omitempty"`
	Properties  []string `json:"properties,omitempty"`
	NormalRange int      `json:"normal_range,omitempty"`
	LongRange   int      `json:"long_range,omitempty"`
}

type ToolDetails struct {
	ToolCategory      string `json:"tool_category,omitempty"`
	ProficiencyNotes  string `json:"proficiency_notes,omitempty"`
}

type MountDetails struct {
	MountType                string `json:"mount_type,omitempty"`
	MovementSpeed            int    `json:"movement_speed,omitempty"`
	CarryingCapacityHundredthsLB int `json:"carrying_capacity_hundredths_lb,omitempty"`
}

type VehicleDetails struct {
	VehicleType              string `json:"vehicle_type,omitempty"`
	MovementSpeed            int    `json:"movement_speed,omitempty"`
	CarryingCapacityHundredthsLB int `json:"carrying_capacity_hundredths_lb,omitempty"`
}

type TreasureDetails struct {
	TreasureKind string `json:"treasure_kind,omitempty"`
}

type ItemDetails struct {
	Container *ContainerDetails `json:"container,omitempty"`
	Armor     *ArmorDetails     `json:"armor,omitempty"`
	Weapon    *WeaponDetails    `json:"weapon,omitempty"`
	Tool      *ToolDetails      `json:"tool,omitempty"`
	Mount     *MountDetails     `json:"mount,omitempty"`
	Vehicle   *VehicleDetails   `json:"vehicle,omitempty"`
	Treasure  *TreasureDetails  `json:"treasure,omitempty"`
}

func (d ItemDetails) JSON() ([]byte, error) {
	return json.Marshal(d)
}

type Item struct {
	ID                 string
	Name               string
	Slug               string
	Description        string
	Category           string
	Subcategory        string
	Rarity             Rarity
	WeightHundredthsLB int
	BaseValueCP        int
	Quantity           int
	IsContainer        bool
	IsStackable        bool
	IsEquipped         bool
	IsMagical          bool
	RequiresAttunement bool
	SourceKind         SourceKind
	Details            ItemDetails
	Location           ItemLocation
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func (i Item) UnitWeightHundredthsLB() int {
	return i.WeightHundredthsLB
}

func (i Item) TotalWeightHundredthsLB() int {
	return i.WeightHundredthsLB * max(i.Quantity, 1)
}

func (i Item) TotalValueCP() int {
	return i.BaseValueCP * max(i.Quantity, 1)
}

func (i Item) ContainerMaxWeightHundredthsLB() int {
	if i.Details.Container == nil {
		return 0
	}
	return i.Details.Container.MaxWeightHundredthsLB
}

func (i Item) Validate() error {
	if strings.TrimSpace(i.Name) == "" {
		return fmt.Errorf("item name is required")
	}
	if i.Quantity < 1 {
		return fmt.Errorf("item quantity must be at least 1")
	}
	if i.WeightHundredthsLB < 0 {
		return fmt.Errorf("item weight cannot be negative")
	}
	if i.BaseValueCP < 0 {
		return fmt.Errorf("item value cannot be negative")
	}
	if !i.Location.Kind.Valid() {
		return fmt.Errorf("item location is invalid")
	}
	return nil
}

func Categories() []string {
	return []string{
		"armor",
		"weapon",
		"equipment",
		"adventuring-gear",
		"tool",
		"mount",
		"vehicle",
		"trade-good",
		"trinket",
		"wondrous-item",
		"potion",
		"scroll",
		"ring",
		"rod",
		"staff",
		"wand",
		"treasure",
		"container",
	}
}

func Rarities() []string {
	return []string{
		string(RarityMundane),
		string(RarityUnknown),
		string(RarityCommon),
		string(RarityUncommon),
		string(RarityRare),
		string(RarityVeryRare),
		string(RarityLegendary),
		string(RarityArtifact),
	}
}

func Slugify(value string) string {
	var builder strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(value)) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			builder.WriteRune(r)
			lastDash = false
		case !lastDash:
			builder.WriteRune('-')
			lastDash = true
		}
	}

	slug := strings.Trim(builder.String(), "-")
	if slug == "" {
		return "item"
	}
	return slug
}

func ParseWeightHundredths(value string) (int, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, nil
	}

	negative := strings.HasPrefix(trimmed, "-")
	if negative {
		trimmed = strings.TrimPrefix(trimmed, "-")
	}

	parts := strings.SplitN(trimmed, ".", 2)
	whole, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("invalid weight %q", value)
	}

	fraction := 0
	if len(parts) == 2 {
		frac := parts[1]
		if len(frac) > 2 {
			return 0, fmt.Errorf("weight %q has too many decimal places", value)
		}
		if len(frac) == 1 {
			frac += "0"
		}
		fraction, err = strconv.Atoi(frac)
		if err != nil {
			return 0, fmt.Errorf("invalid weight %q", value)
		}
	}

	total := whole*100 + fraction
	if negative {
		total *= -1
	}
	return total, nil
}

func FormatWeightHundredths(value int) string {
	sign := ""
	if value < 0 {
		sign = "-"
		value *= -1
	}

	whole := value / 100
	fraction := value % 100
	if fraction == 0 {
		return fmt.Sprintf("%s%d", sign, whole)
	}
	if fraction%10 == 0 {
		return fmt.Sprintf("%s%d.%d", sign, whole, fraction/10)
	}
	return fmt.Sprintf("%s%d.%02d", sign, whole, fraction)
}

func FormatCopperAsGold(valueCP int) string {
	sign := ""
	if valueCP < 0 {
		sign = "-"
		valueCP *= -1
	}

	whole := valueCP / 100
	fraction := valueCP % 100
	if fraction == 0 {
		return fmt.Sprintf("%s%d gp", sign, whole)
	}
	return fmt.Sprintf("%s%d.%02d gp", sign, whole, fraction)
}

func HumanizeLabel(value string) string {
	normalized := strings.TrimSpace(strings.NewReplacer("-", " ", "_", " ").Replace(value))
	if normalized == "" {
		return ""
	}

	words := strings.Fields(normalized)
	for index, word := range words {
		runes := []rune(strings.ToLower(word))
		if len(runes) == 0 {
			continue
		}
		runes[0] = unicode.ToUpper(runes[0])
		words[index] = string(runes)
	}

	return strings.Join(words, " ")
}
