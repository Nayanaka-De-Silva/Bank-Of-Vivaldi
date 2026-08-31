package httpapi

import (
	"strings"

	"bank-of-vivaldi/internal/application"
	"bank-of-vivaldi/internal/domain"
)

// Response DTOs decouple the wire format from the domain/application types, so
// internal refactors don't silently change the API's public shape. Weight
// stays in hundredths-of-a-pound and value in copper pieces — the same units
// the domain uses internally, avoiding lossy float conversion.

type vaultResponse struct {
	ID              string        `json:"id"`
	CharacterName   string        `json:"characterName"`
	StrengthScore   int           `json:"strengthScore"`
	CarryModifierLB int           `json:"carryModifierLb"`
	EncumbranceMode string        `json:"encumbranceMode"`
	Notes           string        `json:"notes"`
	Archived        bool          `json:"archived"`
	Purse           purseResponse `json:"purse"`
}

type purseResponse struct {
	CP int `json:"cp"`
	SP int `json:"sp"`
	EP int `json:"ep"`
	GP int `json:"gp"`
	PP int `json:"pp"`
}

func newVaultResponse(v domain.Vault) vaultResponse {
	return vaultResponse{
		ID:              v.ID,
		CharacterName:   v.CharacterName,
		StrengthScore:   v.StrengthScore,
		CarryModifierLB: v.CarryModifierLB,
		EncumbranceMode: string(v.EncumbranceMode),
		Notes:           v.Notes,
		Archived:        v.Archived,
		Purse: purseResponse{
			CP: v.Purse.CP, SP: v.Purse.SP, EP: v.Purse.EP, GP: v.Purse.GP, PP: v.Purse.PP,
		},
	}
}

type vaultSummaryResponse struct {
	Vault                      vaultResponse `json:"vault"`
	TotalItemWeightHundredths  int           `json:"totalItemWeightHundredths"`
	TotalCarryWeightHundredths int           `json:"totalCarryWeightHundredths"`
	MaxCarryWeightHundredths   int           `json:"maxCarryWeightHundredths"`
	PushDragLiftHundredths     int           `json:"pushDragLiftHundredths"`
	CoinWeightHundredths       int           `json:"coinWeightHundredths"`
	TotalItemValueCP           int           `json:"totalItemValueCp"`
	TotalPurseValueCP          int           `json:"totalPurseValueCp"`
	TotalCombinedValueCP       int           `json:"totalCombinedValueCp"`
	EncumbranceState           string        `json:"encumbranceState"`
}

func newVaultSummaryResponse(s application.VaultSummary) vaultSummaryResponse {
	return vaultSummaryResponse{
		Vault:                      newVaultResponse(s.Vault),
		TotalItemWeightHundredths:  s.TotalItemWeightHundredths,
		TotalCarryWeightHundredths: s.TotalCarryWeightHundredths,
		MaxCarryWeightHundredths:   s.MaxCarryWeightHundredths,
		PushDragLiftHundredths:     s.PushDragLiftHundredths,
		CoinWeightHundredths:       s.CoinWeightHundredths,
		TotalItemValueCP:           s.TotalItemValueCP,
		TotalPurseValueCP:          s.TotalPurseValueCP,
		TotalCombinedValueCP:       s.TotalCombinedValueCP,
		EncumbranceState:           string(s.EncumbranceState),
	}
}

type vaultDetailResponse struct {
	Summary   vaultSummaryResponse `json:"summary"`
	RootItems []itemNodeResponse   `json:"rootItems"`
	Links     []vaultLinkResponse  `json:"links"`
}

type itemNodeResponse struct {
	Item                  itemResponse       `json:"item"`
	Children              []itemNodeResponse `json:"children"`
	TotalWeightHundredths int                `json:"totalWeightHundredths"`
	TotalValueCP          int                `json:"totalValueCp"`
}

func newItemNodeResponse(n application.ItemNode) itemNodeResponse {
	children := make([]itemNodeResponse, len(n.Children))
	for i, child := range n.Children {
		children[i] = newItemNodeResponse(child)
	}
	return itemNodeResponse{
		Item:                  newItemResponse(n.Item),
		Children:              children,
		TotalWeightHundredths: n.TotalWeightHundredths,
		TotalValueCP:          n.TotalValueCP,
	}
}

type vaultLinkResponse struct {
	ID          string `json:"id"`
	VaultID     string `json:"vaultId"`
	ExternalRef string `json:"externalRef"`
	Label       string `json:"label"`
	CreatedAt   string `json:"createdAt"`
}

func newVaultLinkResponse(l domain.VaultLink) vaultLinkResponse {
	return vaultLinkResponse{
		ID:          l.ID,
		VaultID:     l.VaultID,
		ExternalRef: l.ExternalRef,
		Label:       l.Label,
		CreatedAt:   l.CreatedAt.UTC().Format(timeFormat),
	}
}

const timeFormat = "2006-01-02T15:04:05Z07:00"

type itemResponse struct {
	ID                 string               `json:"id"`
	Name               string               `json:"name"`
	Description        string               `json:"description"`
	Category           string               `json:"category"`
	Subcategory        string               `json:"subcategory,omitempty"`
	Rarity             string               `json:"rarity"`
	WeightHundredthsLB int                  `json:"weightHundredthsLb"`
	BaseValueCP        int                  `json:"baseValueCp"`
	Quantity           int                  `json:"quantity"`
	IsContainer        bool                 `json:"isContainer"`
	IsStackable        bool                 `json:"isStackable"`
	IsEquipped         bool                 `json:"isEquipped"`
	IsMagical          bool                 `json:"isMagical"`
	RequiresAttunement bool                 `json:"requiresAttunement"`
	SourceKind         string               `json:"sourceKind"`
	Location           itemLocationResponse `json:"location"`
}

type itemLocationResponse struct {
	Kind                  string `json:"kind"`
	OwnerVaultID          string `json:"ownerVaultId,omitempty"`
	ParentContainerItemID string `json:"parentContainerItemId,omitempty"`
}

func newItemResponse(item domain.Item) itemResponse {
	return itemResponse{
		ID:                 item.ID,
		Name:               item.Name,
		Description:        item.Description,
		Category:           item.Category,
		Subcategory:        item.Subcategory,
		Rarity:             string(item.Rarity),
		WeightHundredthsLB: item.WeightHundredthsLB,
		BaseValueCP:        item.BaseValueCP,
		Quantity:           item.Quantity,
		IsContainer:        item.IsContainer,
		IsStackable:        item.IsStackable,
		IsEquipped:         item.IsEquipped,
		IsMagical:          item.IsMagical,
		RequiresAttunement: item.RequiresAttunement,
		SourceKind:         string(item.SourceKind),
		Location: itemLocationResponse{
			Kind:                  string(item.Location.Kind),
			OwnerVaultID:          item.Location.OwnerVaultID,
			ParentContainerItemID: item.Location.ParentContainerItemID,
		},
	}
}

type itemEntryResponse struct {
	Item           itemResponse `json:"item"`
	ContainerPath  string       `json:"containerPath,omitempty"`
	LocationLabel  string       `json:"locationLabel,omitempty"`
	IsNested       bool         `json:"isNested"`
	TotalWeightLB  string       `json:"totalWeightLb"`
	TotalValueText string       `json:"totalValueText"`
}

func newItemEntryResponse(e application.ItemEntry) itemEntryResponse {
	return itemEntryResponse{
		Item:           newItemResponse(e.Item),
		ContainerPath:  e.ContainerPath,
		LocationLabel:  e.LocationLabel,
		IsNested:       e.IsNested,
		TotalWeightLB:  e.TotalWeightLB,
		TotalValueText: e.TotalValueText,
	}
}

// --- Request DTOs ---

type createVaultRequest struct {
	CharacterName   string `json:"characterName"`
	StrengthScore   int    `json:"strengthScore"`
	CarryModifierLB int    `json:"carryModifierLb"`
	Notes           string `json:"notes"`
}

func (r createVaultRequest) validate() []FieldError {
	var errs []FieldError
	if strings.TrimSpace(r.CharacterName) == "" {
		errs = append(errs, FieldError{Field: "characterName", Message: "is required"})
	}
	if r.StrengthScore < 1 {
		errs = append(errs, FieldError{Field: "strengthScore", Message: "must be at least 1"})
	}
	return errs
}

type linkVaultRequest struct {
	ExternalRef string `json:"externalRef"`
	Label       string `json:"label"`
}

func (r linkVaultRequest) validate() []FieldError {
	var errs []FieldError
	if strings.TrimSpace(r.ExternalRef) == "" {
		errs = append(errs, FieldError{Field: "externalRef", Message: "is required"})
	}
	return errs
}

// addVaultItemRequest is deliberately loosely typed at the JSON level: a
// request either references an existing item (sourceItemId) or defines a new
// one inline. Exactly one shape is expected per request.
type addVaultItemRequest struct {
	// Copy/move an existing item (e.g. from the compendium) into the vault.
	SourceItemID string `json:"sourceItemId,omitempty"`
	Mode         string `json:"mode,omitempty"` // "copy" (default) or "move"

	// Inline item definition, used when SourceItemID is empty.
	Name               string `json:"name,omitempty"`
	Description        string `json:"description,omitempty"`
	Category           string `json:"category,omitempty"`
	Subcategory        string `json:"subcategory,omitempty"`
	Rarity             string `json:"rarity,omitempty"`
	WeightHundredthsLB int    `json:"weightHundredthsLb,omitempty"`
	BaseValueCP        int    `json:"baseValueCp,omitempty"`
	Quantity           int    `json:"quantity,omitempty"`
	IsStackable        bool   `json:"isStackable,omitempty"`

	// ContainerID nests the item/copy inside an existing container in the
	// vault instead of placing it at the vault root.
	ContainerID string `json:"containerId,omitempty"`
}

func (r addVaultItemRequest) isTransfer() bool {
	return strings.TrimSpace(r.SourceItemID) != ""
}

func (r addVaultItemRequest) validate() []FieldError {
	var errs []FieldError
	if r.isTransfer() {
		if r.Mode != "" && r.Mode != "copy" && r.Mode != "move" {
			errs = append(errs, FieldError{Field: "mode", Message: `must be "copy" or "move"`})
		}
		return errs
	}

	if strings.TrimSpace(r.Name) == "" {
		errs = append(errs, FieldError{Field: "name", Message: "is required"})
	}
	if strings.TrimSpace(r.Category) == "" {
		errs = append(errs, FieldError{Field: "category", Message: "is required"})
	} else if !domain.IsValidCategory(strings.ToLower(strings.TrimSpace(r.Category))) {
		errs = append(errs, FieldError{Field: "category", Message: "is not a recognised category"})
	}
	if r.Rarity != "" {
		if _, ok := domain.TryParseRarity(r.Rarity); !ok {
			errs = append(errs, FieldError{Field: "rarity", Message: "is not a recognised rarity"})
		}
	}
	if r.WeightHundredthsLB < 0 {
		errs = append(errs, FieldError{Field: "weightHundredthsLb", Message: "cannot be negative"})
	}
	if r.BaseValueCP < 0 {
		errs = append(errs, FieldError{Field: "baseValueCp", Message: "cannot be negative"})
	}
	if r.Quantity < 0 {
		errs = append(errs, FieldError{Field: "quantity", Message: "cannot be negative"})
	}
	return errs
}
