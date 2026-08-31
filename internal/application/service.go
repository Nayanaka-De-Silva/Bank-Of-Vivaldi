package application

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"bank-of-vivaldi/internal/domain"

	"github.com/google/uuid"
)

type Store interface {
	GetAppSettings(ctx context.Context) (domain.AppSettings, error)
	SetDefaultEncumbranceMode(ctx context.Context, mode domain.EncumbranceMode) error

	CreateVault(ctx context.Context, vault domain.Vault) (domain.Vault, error)
	UpdateVault(ctx context.Context, vault domain.Vault) (domain.Vault, error)
	GetVault(ctx context.Context, id string) (domain.Vault, error)
	ListVaults(ctx context.Context, includeArchived bool) ([]domain.Vault, error)
	SavePurse(ctx context.Context, vaultID string, purse domain.Purse) error

	CreateItem(ctx context.Context, item domain.Item) (domain.Item, error)
	UpdateItem(ctx context.Context, item domain.Item) (domain.Item, error)
	GetItem(ctx context.Context, id string) (domain.Item, error)
	ListItems(ctx context.Context) ([]domain.Item, error)
	DeleteItem(ctx context.Context, id string) error

	CreateVaultLink(ctx context.Context, link domain.VaultLink) (domain.VaultLink, error)
	DeleteVaultLink(ctx context.Context, vaultID, externalRef string) error
	ListVaultLinks(ctx context.Context, vaultID string) ([]domain.VaultLink, error)
}

type Service struct {
	store Store
	now   func() time.Time
}

func NewService(store Store) *Service {
	return &Service{
		store: store,
		now:   time.Now,
	}
}

type VaultSummary struct {
	Vault                      domain.Vault
	TotalItemWeightHundredths  int
	TotalCarryWeightHundredths int
	MaxCarryWeightHundredths   int
	PushDragLiftHundredths     int
	CoinWeightHundredths       int
	TotalItemValueCP           int
	TotalPurseValueCP          int
	TotalCombinedValueCP       int
	EncumbranceState           domain.EncumbranceState
}

type ItemNode struct {
	Item                  domain.Item
	Children              []ItemNode
	TotalWeightHundredths int
	TotalValueCP          int
}

type DashboardData struct {
	Settings            domain.AppSettings
	Vaults              []VaultSummary
	CompendiumWeight    int
	CompendiumValueCP   int
	CompendiumRootItems []ItemNode
}

type VaultDetail struct {
	Settings  domain.AppSettings
	Summary   VaultSummary
	RootItems []ItemNode
}

type ItemDetail struct {
	Settings      domain.AppSettings
	Item          domain.Item
	Children      []ItemNode
	MergeTargets  []domain.Item
	LocationLabel string
}

type SearchFilters struct {
	Query     string
	Category  string
	Rarity    string
	VaultID   string
	Container string
	SortBy    string
}

type SearchResult struct {
	Item           domain.Item
	LocationLabel  string
	ContainerPath  string
	TotalWeightLB  string
	TotalValueText string
}

type SaveItemInput struct {
	ID                 string
	Name               string
	Description        string
	Category           string
	Subcategory        string
	Rarity             domain.Rarity
	WeightHundredthsLB int
	BaseValueCP        int
	Quantity           int
	IsContainer        bool
	IsStackable        bool
	IsEquipped         bool
	IsMagical          bool
	RequiresAttunement bool
	SourceKind         domain.SourceKind
	Details            domain.ItemDetails
	Location           domain.ItemLocation
}

type CreateVaultInput struct {
	CharacterName   string
	StrengthScore   int
	CarryModifierLB int
	Notes           string
}

type UpdateVaultInput struct {
	ID              string
	CharacterName   string
	StrengthScore   int
	CarryModifierLB int
	EncumbranceMode domain.EncumbranceMode
	Notes           string
	Archived        bool
}

type BulkCommitInput struct {
	EncodedRows           string
	LocationKind          domain.LocationKind
	VaultID               string
	ParentContainerItemID string
	IsStackable           bool
	SourceKind            domain.SourceKind
	IsMagical             bool
	RequiresAttunement    bool
	IsEquipped            bool
}

func (s *Service) Settings(ctx context.Context) (domain.AppSettings, error) {
	return s.store.GetAppSettings(ctx)
}

func (s *Service) Dashboard(ctx context.Context) (DashboardData, error) {
	settings, err := s.store.GetAppSettings(ctx)
	if err != nil {
		return DashboardData{}, err
	}

	vaults, err := s.store.ListVaults(ctx, false)
	if err != nil {
		return DashboardData{}, err
	}

	items, err := s.store.ListItems(ctx)
	if err != nil {
		return DashboardData{}, err
	}

	data := DashboardData{
		Settings:          settings,
		Vaults:            summarizeVaults(vaults, items),
		CompendiumWeight:  domain.ComputeCompendiumWeightHundredths(items),
		CompendiumValueCP: domain.ComputeCompendiumValueCP(items),
	}

	compendiumRoots := filterRootItems(items, domain.LocationKindCompendiumRoot, "")
	data.CompendiumRootItems = buildNodes(compendiumRoots, items)

	return data, nil
}

func (s *Service) ListVaults(ctx context.Context) ([]VaultSummary, domain.AppSettings, error) {
	settings, err := s.store.GetAppSettings(ctx)
	if err != nil {
		return nil, domain.AppSettings{}, err
	}

	vaults, err := s.store.ListVaults(ctx, true)
	if err != nil {
		return nil, domain.AppSettings{}, err
	}

	items, err := s.store.ListItems(ctx)
	if err != nil {
		return nil, domain.AppSettings{}, err
	}

	return summarizeVaults(vaults, items), settings, nil
}

func (s *Service) CreateVault(ctx context.Context, input CreateVaultInput) (domain.Vault, error) {
	if strings.TrimSpace(input.CharacterName) == "" {
		return domain.Vault{}, fmt.Errorf("character name is required")
	}
	if input.StrengthScore < 1 {
		return domain.Vault{}, fmt.Errorf("strength score must be at least 1")
	}

	settings, err := s.store.GetAppSettings(ctx)
	if err != nil {
		return domain.Vault{}, err
	}

	now := s.now().UTC()
	vault := domain.Vault{
		ID:              uuid.NewString(),
		CharacterName:   strings.TrimSpace(input.CharacterName),
		StrengthScore:   input.StrengthScore,
		CarryModifierLB: input.CarryModifierLB,
		EncumbranceMode: settings.DefaultEncumbranceMode,
		Notes:           strings.TrimSpace(input.Notes),
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	return s.store.CreateVault(ctx, vault)
}

func (s *Service) UpdateVault(ctx context.Context, input UpdateVaultInput) (domain.Vault, error) {
	vault, err := s.store.GetVault(ctx, input.ID)
	if err != nil {
		return domain.Vault{}, err
	}

	if strings.TrimSpace(input.CharacterName) == "" {
		return domain.Vault{}, fmt.Errorf("character name is required")
	}
	if input.StrengthScore < 1 {
		return domain.Vault{}, fmt.Errorf("strength score must be at least 1")
	}

	vault.CharacterName = strings.TrimSpace(input.CharacterName)
	vault.StrengthScore = input.StrengthScore
	vault.CarryModifierLB = input.CarryModifierLB
	vault.EncumbranceMode = domain.ParseEncumbranceMode(string(input.EncumbranceMode))
	vault.Notes = strings.TrimSpace(input.Notes)
	vault.Archived = input.Archived
	vault.UpdatedAt = s.now().UTC()

	return s.store.UpdateVault(ctx, vault)
}

func (s *Service) GetVaultDetail(ctx context.Context, vaultID string) (VaultDetail, error) {
	settings, err := s.store.GetAppSettings(ctx)
	if err != nil {
		return VaultDetail{}, err
	}

	vault, err := s.store.GetVault(ctx, vaultID)
	if err != nil {
		return VaultDetail{}, err
	}

	items, err := s.store.ListItems(ctx)
	if err != nil {
		return VaultDetail{}, err
	}

	summary := summarizeVaults([]domain.Vault{vault}, items)[0]
	roots := filterRootItems(items, domain.LocationKindVaultRoot, vault.ID)

	return VaultDetail{
		Settings:  settings,
		Summary:   summary,
		RootItems: buildNodes(roots, items),
	}, nil
}

func (s *Service) AdjustPurse(ctx context.Context, vaultID string, purse domain.Purse) error {
	if _, err := s.store.GetVault(ctx, vaultID); err != nil {
		return err
	}
	if purse.CP < 0 || purse.SP < 0 || purse.EP < 0 || purse.GP < 0 || purse.PP < 0 {
		return fmt.Errorf("coin counts cannot be negative")
	}
	return s.store.SavePurse(ctx, vaultID, purse)
}

func (s *Service) SetDefaultEncumbranceMode(ctx context.Context, mode domain.EncumbranceMode) error {
	mode = domain.ParseEncumbranceMode(string(mode))
	if err := s.store.SetDefaultEncumbranceMode(ctx, mode); err != nil {
		return err
	}

	vaults, err := s.store.ListVaults(ctx, false)
	if err != nil {
		return err
	}

	for _, vault := range vaults {
		vault.EncumbranceMode = mode
		vault.UpdatedAt = s.now().UTC()
		if _, err := s.store.UpdateVault(ctx, vault); err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) SaveItem(ctx context.Context, input SaveItemInput) (domain.Item, error) {
	allItems, err := s.store.ListItems(ctx)
	if err != nil {
		return domain.Item{}, err
	}

	item := domain.Item{
		ID:                 input.ID,
		Name:               strings.TrimSpace(input.Name),
		Slug:               domain.Slugify(input.Name),
		Description:        strings.TrimSpace(input.Description),
		Category:           strings.ToLower(strings.TrimSpace(input.Category)),
		Subcategory:        strings.ToLower(strings.TrimSpace(input.Subcategory)),
		Rarity:             domain.ParseRarity(string(input.Rarity)),
		WeightHundredthsLB: input.WeightHundredthsLB,
		BaseValueCP:        input.BaseValueCP,
		Quantity:           input.Quantity,
		IsContainer:        input.IsContainer || input.Details.Container != nil,
		IsStackable:        input.IsStackable,
		IsEquipped:         input.IsEquipped,
		IsMagical:          input.IsMagical,
		RequiresAttunement: input.RequiresAttunement,
		SourceKind:         domain.ParseSourceKind(string(input.SourceKind)),
		Details:            input.Details,
		Location:           input.Location,
	}

	if item.ID == "" {
		item.ID = uuid.NewString()
	}
	if item.Category == "" {
		item.Category = "equipment"
	}
	if item.Quantity < 1 {
		item.Quantity = 1
	}
	if item.SourceKind == "" {
		item.SourceKind = domain.SourceKindManual
	}
	if item.Rarity == "" {
		item.Rarity = domain.RarityMundane
	}

	now := s.now().UTC()
	existingIndex := slices.IndexFunc(allItems, func(candidate domain.Item) bool {
		return candidate.ID == item.ID
	})
	if existingIndex >= 0 {
		item.CreatedAt = allItems[existingIndex].CreatedAt
		item.UpdatedAt = now
	} else {
		item.CreatedAt = now
		item.UpdatedAt = now
	}

	if err := item.Validate(); err != nil {
		return domain.Item{}, err
	}

	if err := s.resolveLocation(&item, allItems); err != nil {
		return domain.Item{}, err
	}
	if err := s.validateCircularMove(item, allItems); err != nil {
		return domain.Item{}, err
	}

	simulated, err := s.simulateMutation(allItems, item)
	if err != nil {
		return domain.Item{}, err
	}
	if err := s.validateSimulation(ctx, simulated, item); err != nil {
		return domain.Item{}, err
	}

	var saved domain.Item
	if existingIndex >= 0 {
		saved, err = s.store.UpdateItem(ctx, item)
	} else {
		saved, err = s.store.CreateItem(ctx, item)
	}
	if err != nil {
		return domain.Item{}, err
	}

	if err := s.persistDescendantLocationUpdates(ctx, allItems, simulated, item.ID); err != nil {
		return domain.Item{}, err
	}

	return saved, nil
}

func (s *Service) GetItemDetail(ctx context.Context, id string) (ItemDetail, error) {
	settings, err := s.store.GetAppSettings(ctx)
	if err != nil {
		return ItemDetail{}, err
	}

	item, err := s.store.GetItem(ctx, id)
	if err != nil {
		return ItemDetail{}, err
	}

	items, err := s.store.ListItems(ctx)
	if err != nil {
		return ItemDetail{}, err
	}

	var children []ItemNode
	if item.IsContainer {
		roots := directChildren(items, item.ID)
		children = buildNodes(roots, items)
	}

	mergeTargets := make([]domain.Item, 0)
	for _, candidate := range items {
		if candidate.ID == item.ID || !candidate.IsStackable {
			continue
		}
		if sameLocation(candidate.Location, item.Location) && domain.ItemsMergeable(candidate, item) {
			mergeTargets = append(mergeTargets, candidate)
		}
	}

	return ItemDetail{
		Settings:      settings,
		Item:          item,
		Children:      children,
		MergeTargets:  mergeTargets,
		LocationLabel: s.locationLabel(item, items),
	}, nil
}

func (s *Service) DeleteItem(ctx context.Context, id string) error {
	items, err := s.store.ListItems(ctx)
	if err != nil {
		return err
	}

	for _, item := range items {
		if item.Location.ParentContainerItemID == id {
			return fmt.Errorf("cannot delete a container that still has child items")
		}
	}

	return s.store.DeleteItem(ctx, id)
}

func (s *Service) MoveItem(ctx context.Context, itemID string, location domain.ItemLocation) (domain.Item, error) {
	item, err := s.store.GetItem(ctx, itemID)
	if err != nil {
		return domain.Item{}, err
	}
	item.Location = location
	return s.SaveItem(ctx, toSaveInput(item))
}

// CopyItem deep-copies the item subtree rooted at itemID and places the root
// clone at location. The full subtree is validated as a single unit before any
// row is written — a per-node approach would commit partial subtrees and leave
// orphans if a later node fails capacity checks. With no Store transaction this
// call is not atomic: a process death mid-write can leave orphaned rows, but
// persistClones attempts best-effort cleanup on error.
func (s *Service) CopyItem(ctx context.Context, itemID string, location domain.ItemLocation) (domain.Item, error) {
	allItems, err := s.store.ListItems(ctx)
	if err != nil {
		return domain.Item{}, err
	}

	src, ok := findItem(allItems, itemID)
	if !ok {
		return domain.Item{}, fmt.Errorf("item %q: %w", itemID, domain.ErrNotFound)
	}

	now := s.now().UTC()
	_, childrenMap := domain.BuildItemIndexes(allItems)
	// build clone tree in pre-order so parent IDs are known before children
	clones := cloneSubtree(src, "", childrenMap, now, map[string]bool{})

	clones[0].Location = location
	if err := clones[0].Validate(); err != nil {
		return domain.Item{}, err
	}
	if err := s.resolveLocation(&clones[0], allItems); err != nil {
		return domain.Item{}, err
	}
	if err := s.validateCircularMove(clones[0], allItems); err != nil {
		return domain.Item{}, err
	}

	// propagate resolved vault owner to every descendant clone (O(n), no fixpoint loop needed)
	for i := range clones[1:] {
		clones[i+1].Location.OwnerVaultID = clones[0].Location.OwnerVaultID
	}

	simulated := append(domain.CloneItems(allItems), clones...)
	// validate all clones as one unit — prevents partial-subtree capacity bypasses
	if err := s.validateSimulationForItems(ctx, simulated, clones); err != nil {
		return domain.Item{}, err
	}

	if err := s.persistClones(ctx, clones); err != nil {
		return domain.Item{}, err
	}

	return clones[0], nil
}

// cloneSubtree recursively builds a pre-order slice of deep-copied items.
// parentCloneID is empty for the root; descendants get the new parent's ID so
// the pre-order slice satisfies the parent-before-child FK constraint.
func cloneSubtree(src domain.Item, parentCloneID string, children map[string][]domain.Item, now time.Time, visited map[string]bool) []domain.Item {
	if visited[src.ID] {
		return nil
	}
	visited[src.ID] = true

	clone := src
	clone.ID = uuid.NewString()
	clone.Details = domain.CloneItemDetails(src.Details)
	clone.CreatedAt = now
	clone.UpdatedAt = now
	clone.IsEquipped = false
	if parentCloneID != "" {
		clone.Location = domain.ItemLocation{
			Kind:                  domain.LocationKindContainer,
			ParentContainerItemID: parentCloneID,
		}
	}

	result := []domain.Item{clone}
	for _, child := range children[src.ID] {
		result = append(result, cloneSubtree(child, clone.ID, children, now, visited)...)
	}
	return result
}

// persistClones writes clones to the store in the order given (pre-order, satisfying FK).
// On any failure it deletes already-created rows in reverse order; if cleanup itself
// fails, the IDs of surviving rows are included in the returned error.
func (s *Service) persistClones(ctx context.Context, clones []domain.Item) error {
	created := make([]string, 0, len(clones))
	for _, clone := range clones {
		if _, err := s.store.CreateItem(ctx, clone); err != nil {
			var orphaned []string
			for i := len(created) - 1; i >= 0; i-- {
				if cleanupErr := s.store.DeleteItem(ctx, created[i]); cleanupErr != nil {
					orphaned = append(orphaned, created[i])
				}
			}
			if len(orphaned) > 0 {
				return fmt.Errorf("persist failed (%w); orphaned rows: %v", err, orphaned)
			}
			return err
		}
		created = append(created, clone.ID)
	}
	return nil
}

func (s *Service) SplitStack(ctx context.Context, itemID string, quantity int) error {
	if quantity < 1 {
		return fmt.Errorf("split quantity must be at least 1")
	}

	item, err := s.store.GetItem(ctx, itemID)
	if err != nil {
		return err
	}
	if item.Quantity <= quantity {
		return fmt.Errorf("split quantity must be smaller than current quantity")
	}

	item.Quantity -= quantity
	if _, err := s.store.UpdateItem(ctx, item); err != nil {
		return err
	}

	newItem := item
	newItem.ID = uuid.NewString()
	newItem.Quantity = quantity
	now := s.now().UTC()
	newItem.CreatedAt = now
	newItem.UpdatedAt = now
	_, err = s.store.CreateItem(ctx, newItem)
	return err
}

func (s *Service) MergeStacks(ctx context.Context, sourceID, targetID string) error {
	source, err := s.store.GetItem(ctx, sourceID)
	if err != nil {
		return err
	}
	target, err := s.store.GetItem(ctx, targetID)
	if err != nil {
		return err
	}
	if !sameLocation(source.Location, target.Location) {
		return fmt.Errorf("items must be in the same location to merge")
	}
	if !domain.ItemsMergeable(source, target) {
		return fmt.Errorf("items do not share the same effective properties")
	}

	target.Quantity += source.Quantity
	target.UpdatedAt = s.now().UTC()
	if _, err := s.store.UpdateItem(ctx, target); err != nil {
		return err
	}
	return s.store.DeleteItem(ctx, source.ID)
}

// PreviewBulk parses the bulk-import text and resolves any vault/container names
// in the rows against the store. Name resolution failures append row errors rather
// than returning a top-level error, so callers always get a BulkPreview to render.
func (s *Service) PreviewBulk(ctx context.Context, text string, defaults domain.BulkDefaults) (domain.BulkPreview, error) {
	preview := domain.ParseBulkItemInput(text, defaults)

	vaults, err := s.AllVaults(ctx)
	if err != nil {
		return domain.BulkPreview{}, err
	}
	allItems, err := s.store.ListItems(ctx)
	if err != nil {
		return domain.BulkPreview{}, err
	}

	vaultsByName := make(map[string][]domain.Vault)
	for _, v := range vaults {
		key := strings.ToLower(v.CharacterName)
		vaultsByName[key] = append(vaultsByName[key], v)
	}

	itemsByName := make(map[string][]domain.Item)
	for _, item := range allItems {
		key := strings.ToLower(item.Name)
		itemsByName[key] = append(itemsByName[key], item)
	}

	for i := range preview.Rows {
		row := &preview.Rows[i]

		if row.VaultName != "" {
			key := strings.ToLower(row.VaultName)
			matches := vaultsByName[key]
			switch len(matches) {
			case 0:
				row.Errors = append(row.Errors, fmt.Sprintf("vault %q not found", row.VaultName))
			case 1:
				row.ResolvedVaultID = matches[0].ID
			default:
				row.Errors = append(row.Errors, fmt.Sprintf("vault name %q is ambiguous (%d matches)", row.VaultName, len(matches)))
			}
		}

		if row.ContainerName != "" {
			key := strings.ToLower(row.ContainerName)
			matches := itemsByName[key]
			switch len(matches) {
			case 0:
				row.Errors = append(row.Errors, fmt.Sprintf("container %q not found", row.ContainerName))
			case 1:
				if !matches[0].IsContainer {
					row.Errors = append(row.Errors, fmt.Sprintf("item %q is not a container", row.ContainerName))
				} else {
					row.ResolvedContainerID = matches[0].ID
				}
			default:
				// Among multiple name matches, count those that are actually containers.
				var containerMatches []domain.Item
				for _, m := range matches {
					if m.IsContainer {
						containerMatches = append(containerMatches, m)
					}
				}
				switch len(containerMatches) {
				case 0:
					row.Errors = append(row.Errors, fmt.Sprintf("item %q is not a container", row.ContainerName))
				case 1:
					row.ResolvedContainerID = containerMatches[0].ID
				default:
					row.Errors = append(row.Errors, fmt.Sprintf("container name %q is ambiguous (%d matches)", row.ContainerName, len(containerMatches)))
				}
			}
		}
	}

	return preview, nil
}

func (s *Service) EncodeBulkRows(rows []domain.BulkPreviewRow) (string, error) {
	bytes, err := json.Marshal(rows)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(bytes), nil
}

func (s *Service) CommitBulk(ctx context.Context, input BulkCommitInput) error {
	if input.EncodedRows == "" {
		return fmt.Errorf("bulk preview payload is required")
	}

	payload, err := base64.StdEncoding.DecodeString(input.EncodedRows)
	if err != nil {
		return fmt.Errorf("invalid bulk preview payload")
	}

	var rows []domain.BulkPreviewRow
	if err := json.Unmarshal(payload, &rows); err != nil {
		return fmt.Errorf("invalid bulk preview rows")
	}

	for _, row := range rows {
		if len(row.Errors) > 0 {
			return fmt.Errorf("bulk preview still contains invalid rows")
		}
	}

	// Resolve every row's effective location before committing anything. A
	// row's location=vault/location=container attribute passes preview
	// syntactically valid (PreviewBulk only checks it against LocationKind's
	// three known values) but still needs an actual vault/container target,
	// which depends on the batch defaults chosen at commit time and so can't
	// be checked until now. Failing fast here — before any SaveItem call —
	// keeps a bad row from leaving earlier rows in the same batch committed;
	// CommitBulk has no transaction, so a failure discovered mid-loop would
	// otherwise partially apply the batch.
	locations := make([]domain.ItemLocation, len(rows))
	for i, row := range rows {
		location, err := resolveBulkRowLocation(row, input)
		if err != nil {
			return err
		}
		locations[i] = location
	}

	for i, row := range rows {
		// Per-row SourceKind overrides batch default; batch defaults to manual.
		sourceKind := input.SourceKind
		if sourceKind == "" {
			sourceKind = domain.SourceKindManual
		}
		if row.SourceKind != "" {
			sourceKind = row.SourceKind
		}

		_, err := s.SaveItem(ctx, SaveItemInput{
			Name:               row.Name,
			Description:        row.Description,
			Category:           row.Category,
			Subcategory:        row.Subcategory,
			Rarity:             row.Rarity,
			WeightHundredthsLB: row.WeightHundredthsLB,
			BaseValueCP:        row.BaseValueCP,
			Quantity:           row.Quantity,
			IsStackable:        boolOr(row.IsStackable, input.IsStackable),
			IsContainer:        boolOr(row.IsContainer, false),
			IsMagical:          boolOr(row.IsMagical, input.IsMagical),
			RequiresAttunement: boolOr(row.RequiresAttunement, input.RequiresAttunement),
			IsEquipped:         boolOr(row.IsEquipped, input.IsEquipped),
			SourceKind:         sourceKind,
			Details:            row.Details,
			Location:           locations[i],
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// resolveBulkRowLocation computes a row's effective placement, applying the
// same per-row-overrides-batch-default precedence as the rest of CommitBulk:
// a resolved container name wins, then a resolved vault name, then an
// explicit location= attribute, else the batch default. It returns an error
// naming the offending row when the resolved kind requires a target
// (vault/container) that isn't actually available.
func resolveBulkRowLocation(row domain.BulkPreviewRow, input BulkCommitInput) (domain.ItemLocation, error) {
	locationKind := input.LocationKind
	vaultID := input.VaultID
	containerID := input.ParentContainerItemID

	if row.ResolvedContainerID != "" {
		locationKind = domain.LocationKindContainer
		containerID = row.ResolvedContainerID
		if row.ResolvedVaultID != "" {
			vaultID = row.ResolvedVaultID
		}
	} else if row.ResolvedVaultID != "" {
		locationKind = domain.LocationKindVaultRoot
		vaultID = row.ResolvedVaultID
		containerID = ""
	} else if row.LocationKind != "" {
		locationKind = row.LocationKind
	}

	switch locationKind {
	case domain.LocationKindVaultRoot:
		if vaultID == "" {
			return domain.ItemLocation{}, fmt.Errorf("row %d (%q): location=vault requires a vault (set vault= on the row or choose a default vault)", row.LineNumber, row.Name)
		}
	case domain.LocationKindContainer:
		if containerID == "" {
			return domain.ItemLocation{}, fmt.Errorf("row %d (%q): location=container requires a target container (set parent= on the row or choose a default container)", row.LineNumber, row.Name)
		}
	case domain.LocationKindCompendiumRoot:
		// No target required.
	default:
		return domain.ItemLocation{}, fmt.Errorf("row %d (%q): invalid location kind %q", row.LineNumber, row.Name, locationKind)
	}

	return domain.ItemLocation{
		Kind:                  locationKind,
		OwnerVaultID:          vaultID,
		ParentContainerItemID: containerID,
	}, nil
}

// boolOr returns the value pointed to by override if non-nil, otherwise fallback.
// It is used to merge per-row pointer bools with batch-level boolean defaults.
func boolOr(override *bool, fallback bool) bool {
	if override != nil {
		return *override
	}
	return fallback
}

func (s *Service) SearchInventory(ctx context.Context, filters SearchFilters) ([]SearchResult, domain.AppSettings, error) {
	settings, err := s.store.GetAppSettings(ctx)
	if err != nil {
		return nil, domain.AppSettings{}, err
	}
	items, err := s.store.ListItems(ctx)
	if err != nil {
		return nil, domain.AppSettings{}, err
	}
	vaults, err := s.store.ListVaults(ctx, true)
	if err != nil {
		return nil, domain.AppSettings{}, err
	}

	vaultNames := map[string]string{}
	for _, vault := range vaults {
		vaultNames[vault.ID] = vault.CharacterName
	}

	results := make([]SearchResult, 0, len(items))
	query := strings.ToLower(strings.TrimSpace(filters.Query))
	category := strings.ToLower(strings.TrimSpace(filters.Category))
	rarity := strings.ToLower(strings.TrimSpace(filters.Rarity))
	containerText := strings.ToLower(strings.TrimSpace(filters.Container))

	for _, item := range items {
		if filters.VaultID != "" && item.Location.OwnerVaultID != filters.VaultID {
			continue
		}
		if category != "" && strings.ToLower(item.Category) != category {
			continue
		}
		if rarity != "" && string(item.Rarity) != rarity {
			continue
		}

		containerPath := buildContainerPath(item, items)
		haystack := strings.ToLower(strings.Join([]string{
			item.Name,
			item.Category,
			item.Subcategory,
			string(item.Rarity),
			vaultNames[item.Location.OwnerVaultID],
			containerPath,
		}, " "))

		if query != "" && !strings.Contains(haystack, query) {
			continue
		}
		if containerText != "" && !strings.Contains(strings.ToLower(containerPath), containerText) {
			continue
		}

		results = append(results, SearchResult{
			Item:           item,
			LocationLabel:  s.locationLabel(item, items),
			ContainerPath:  containerPath,
			TotalWeightLB:  domain.FormatWeightHundredths(item.TotalWeightHundredthsLB()),
			TotalValueText: domain.FormatCopperAsGold(item.TotalValueCP()),
		})
	}

	sortSearchResults(results, filters.SortBy)
	return results, settings, nil
}

func (s *Service) AllVaults(ctx context.Context) ([]domain.Vault, error) {
	return s.store.ListVaults(ctx, true)
}

func (s *Service) ListContainers(ctx context.Context) ([]domain.Item, error) {
	items, err := s.store.ListItems(ctx)
	if err != nil {
		return nil, err
	}

	containers := make([]domain.Item, 0)
	for _, item := range items {
		if item.IsContainer {
			containers = append(containers, item)
		}
	}

	sort.Slice(containers, func(i, j int) bool {
		return strings.ToLower(containers[i].Name) < strings.ToLower(containers[j].Name)
	})
	return containers, nil
}

func summarizeVaults(vaults []domain.Vault, items []domain.Item) []VaultSummary {
	summaries := make([]VaultSummary, 0, len(vaults))
	for _, vault := range vaults {
		totalItemWeight := domain.ComputeVaultItemWeightHundredths(items, vault.ID)
		totalPurseWeight := domain.ComputeCoinWeightHundredthsLB(vault.Purse)
		totalCarry := totalItemWeight + totalPurseWeight
		totalItemValue := domain.ComputeVaultItemValueCP(items, vault.ID)
		totalPurseValue := domain.ComputePurseValueCP(vault.Purse)

		summaries = append(summaries, VaultSummary{
			Vault:                      vault,
			TotalItemWeightHundredths:  totalItemWeight,
			TotalCarryWeightHundredths: totalCarry,
			MaxCarryWeightHundredths:   vault.CarryCapacityHundredthsLB(),
			PushDragLiftHundredths:     vault.PushDragLiftHundredthsLB(),
			CoinWeightHundredths:       totalPurseWeight,
			TotalItemValueCP:           totalItemValue,
			TotalPurseValueCP:          totalPurseValue,
			TotalCombinedValueCP:       totalItemValue + totalPurseValue,
			EncumbranceState:           domain.GetEncumbranceState(totalCarry, vault),
		})
	}

	sort.Slice(summaries, func(i, j int) bool {
		return strings.ToLower(summaries[i].Vault.CharacterName) < strings.ToLower(summaries[j].Vault.CharacterName)
	})

	return summaries
}

func buildNodes(roots []domain.Item, allItems []domain.Item) []ItemNode {
	byID, children := domain.BuildItemIndexes(allItems)
	nodes := make([]ItemNode, 0, len(roots))
	for _, root := range roots {
		nodes = append(nodes, buildNode(root, byID, children))
	}
	return nodes
}

func buildNode(item domain.Item, byID map[string]domain.Item, children map[string][]domain.Item) ItemNode {
	node := ItemNode{
		Item:                  item,
		TotalWeightHundredths: domain.ComputeSubtreeWeightHundredths(item.ID, byID, children),
		TotalValueCP:          domain.ComputeSubtreeValueCP(item.ID, byID, children),
	}
	for _, child := range children[item.ID] {
		node.Children = append(node.Children, buildNode(child, byID, children))
	}
	return node
}

func filterRootItems(items []domain.Item, kind domain.LocationKind, ownerVaultID string) []domain.Item {
	roots := make([]domain.Item, 0)
	for _, item := range items {
		if item.Location.Kind == kind && item.Location.OwnerVaultID == ownerVaultID && item.Location.ParentContainerItemID == "" {
			roots = append(roots, item)
		}
	}

	sort.Slice(roots, func(i, j int) bool {
		return strings.ToLower(roots[i].Name) < strings.ToLower(roots[j].Name)
	})
	return roots
}

func directChildren(items []domain.Item, parentID string) []domain.Item {
	children := make([]domain.Item, 0)
	for _, item := range items {
		if item.Location.ParentContainerItemID == parentID {
			children = append(children, item)
		}
	}

	sort.Slice(children, func(i, j int) bool {
		return strings.ToLower(children[i].Name) < strings.ToLower(children[j].Name)
	})
	return children
}

func (s *Service) resolveLocation(item *domain.Item, allItems []domain.Item) error {
	item.Location.Kind = domain.LocationKind(strings.TrimSpace(string(item.Location.Kind)))
	switch item.Location.Kind {
	case domain.LocationKindCompendiumRoot:
		item.Location.OwnerVaultID = ""
		item.Location.ParentContainerItemID = ""
	case domain.LocationKindVaultRoot:
		if item.Location.OwnerVaultID == "" {
			return fmt.Errorf("vault destination requires a vault: %w", domain.ErrInvalidInput)
		}
		item.Location.ParentContainerItemID = ""
	case domain.LocationKindContainer:
		if item.Location.ParentContainerItemID == "" {
			return fmt.Errorf("container destination requires a target container: %w", domain.ErrInvalidInput)
		}
		parent, ok := findItem(allItems, item.Location.ParentContainerItemID)
		if !ok {
			return fmt.Errorf("target container %q: %w", item.Location.ParentContainerItemID, domain.ErrNotFound)
		}
		if !parent.IsContainer {
			return fmt.Errorf("target item %q is not a container: %w", parent.ID, domain.ErrInvalidInput)
		}
		item.Location.OwnerVaultID = parent.Location.OwnerVaultID
	default:
		return fmt.Errorf("unsupported location kind %q: %w", item.Location.Kind, domain.ErrInvalidInput)
	}

	return nil
}

func (s *Service) validateCircularMove(item domain.Item, items []domain.Item) error {
	if item.Location.Kind != domain.LocationKindContainer || item.Location.ParentContainerItemID == "" {
		return nil
	}

	parentID := item.Location.ParentContainerItemID
	if parentID == item.ID {
		return fmt.Errorf("a container cannot contain itself")
	}

	for parentID != "" {
		if parentID == item.ID {
			return fmt.Errorf("a container cannot be moved into its own descendant")
		}
		parent, ok := findItem(items, parentID)
		if !ok {
			return nil
		}
		parentID = parent.Location.ParentContainerItemID
	}

	return nil
}

func (s *Service) simulateMutation(allItems []domain.Item, item domain.Item) ([]domain.Item, error) {
	cloned := domain.CloneItems(allItems)
	index := slices.IndexFunc(cloned, func(candidate domain.Item) bool {
		return candidate.ID == item.ID
	})
	if index >= 0 {
		cloned[index] = item
	} else {
		cloned = append(cloned, item)
	}

	s.syncDescendantOwnership(cloned, item.ID, item.Location.OwnerVaultID)
	return cloned, nil
}

func (s *Service) syncDescendantOwnership(items []domain.Item, rootID, ownerVaultID string) {
	for changed := true; changed; {
		changed = false
		for idx, item := range items {
			if item.Location.ParentContainerItemID == rootID && item.Location.OwnerVaultID != ownerVaultID {
				items[idx].Location.OwnerVaultID = ownerVaultID
				changed = true
				s.syncDescendantOwnership(items, item.ID, ownerVaultID)
			}
		}
	}
}

func (s *Service) persistDescendantLocationUpdates(ctx context.Context, originalItems, simulatedItems []domain.Item, rootID string) error {
	originalByID := make(map[string]domain.Item, len(originalItems))
	for _, item := range originalItems {
		originalByID[item.ID] = item
	}

	for _, item := range simulatedItems {
		if item.ID == rootID || !isDescendantOf(item.ID, rootID, simulatedItems) {
			continue
		}

		original, ok := originalByID[item.ID]
		if !ok {
			continue
		}

		if sameLocation(original.Location, item.Location) {
			continue
		}

		if _, err := s.store.UpdateItem(ctx, item); err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) validateSimulationForItems(ctx context.Context, items []domain.Item, targets []domain.Item) error {
	byID, children := domain.BuildItemIndexes(items)

	parentIDs := map[string]struct{}{}
	containerIDs := map[string]struct{}{}
	vaultIDs := map[string]struct{}{}
	for _, target := range targets {
		if target.Location.Kind == domain.LocationKindContainer {
			parentIDs[target.Location.ParentContainerItemID] = struct{}{}
		}
		if target.IsContainer {
			containerIDs[target.ID] = struct{}{}
		}
		if target.Location.OwnerVaultID != "" {
			vaultIDs[target.Location.OwnerVaultID] = struct{}{}
		}
	}

	for id := range parentIDs {
		container, ok := byID[id]
		if !ok {
			return fmt.Errorf("target container %q: %w", id, domain.ErrNotFound)
		}
		if err := domain.ValidateContainerCapacity(container, domain.ComputeContainedWeightHundredths(container.ID, byID, children)); err != nil {
			return err
		}
	}

	for id := range containerIDs {
		container, ok := byID[id]
		if !ok {
			continue
		}
		if err := domain.ValidateContainerCapacity(container, domain.ComputeContainedWeightHundredths(container.ID, byID, children)); err != nil {
			return err
		}
	}

	for id := range vaultIDs {
		vault, err := s.store.GetVault(ctx, id)
		if err != nil {
			return err
		}
		totalWeight := domain.ComputeVaultItemWeightHundredths(items, vault.ID) + domain.ComputeCoinWeightHundredthsLB(vault.Purse)
		if err := domain.ValidateVaultCapacity(vault, totalWeight); err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) validateSimulation(ctx context.Context, items []domain.Item, target domain.Item) error {
	return s.validateSimulationForItems(ctx, items, []domain.Item{target})
}

func (s *Service) locationLabel(item domain.Item, allItems []domain.Item) string {
	switch item.Location.Kind {
	case domain.LocationKindCompendiumRoot:
		return "Compendium"
	case domain.LocationKindVaultRoot:
		return fmt.Sprintf("Vault %s", item.Location.OwnerVaultID)
	case domain.LocationKindContainer:
		return buildContainerPath(item, allItems)
	default:
		return "Unknown"
	}
}

func buildContainerPath(item domain.Item, allItems []domain.Item) string {
	if item.Location.ParentContainerItemID == "" {
		if item.Location.OwnerVaultID == "" {
			return "Compendium"
		}
		return fmt.Sprintf("Vault %s", item.Location.OwnerVaultID)
	}

	parts := []string{}
	parentID := item.Location.ParentContainerItemID
	for parentID != "" {
		parent, ok := findItem(allItems, parentID)
		if !ok {
			break
		}
		parts = append([]string{parent.Name}, parts...)
		parentID = parent.Location.ParentContainerItemID
	}
	return strings.Join(parts, " / ")
}

func sortSearchResults(results []SearchResult, sortBy string) {
	sort.SliceStable(results, func(i, j int) bool {
		return itemLess(results[i].Item, results[j].Item, sortBy)
	})
}

// itemLess is the shared ordering used by every item listing so search and the
// compendium browser cannot drift apart.
func itemLess(a, b domain.Item, sortBy string) bool {
	switch sortBy {
	case "weight":
		return a.TotalWeightHundredthsLB() < b.TotalWeightHundredthsLB()
	case "value":
		return a.TotalValueCP() < b.TotalValueCP()
	case "category":
		return a.Category < b.Category
	case "rarity":
		return string(a.Rarity) < string(b.Rarity)
	case "updated":
		return a.UpdatedAt.After(b.UpdatedAt)
	default:
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	}
}

func sameLocation(a, b domain.ItemLocation) bool {
	return a.Kind == b.Kind &&
		a.OwnerVaultID == b.OwnerVaultID &&
		a.ParentContainerItemID == b.ParentContainerItemID
}

func findItem(items []domain.Item, id string) (domain.Item, bool) {
	for _, item := range items {
		if item.ID == id {
			return item, true
		}
	}
	return domain.Item{}, false
}

func isDescendantOf(candidateID, rootID string, items []domain.Item) bool {
	current, ok := findItem(items, candidateID)
	if !ok {
		return false
	}

	parentID := current.Location.ParentContainerItemID
	for parentID != "" {
		if parentID == rootID {
			return true
		}
		parent, ok := findItem(items, parentID)
		if !ok {
			return false
		}
		parentID = parent.Location.ParentContainerItemID
	}

	return false
}

func toSaveInput(item domain.Item) SaveItemInput {
	return SaveItemInput{
		ID:                 item.ID,
		Name:               item.Name,
		Description:        item.Description,
		Category:           item.Category,
		Subcategory:        item.Subcategory,
		Rarity:             item.Rarity,
		WeightHundredthsLB: item.WeightHundredthsLB,
		BaseValueCP:        item.BaseValueCP,
		Quantity:           item.Quantity,
		IsContainer:        item.IsContainer,
		IsStackable:        item.IsStackable,
		IsEquipped:         item.IsEquipped,
		IsMagical:          item.IsMagical,
		RequiresAttunement: item.RequiresAttunement,
		SourceKind:         item.SourceKind,
		Details:            item.Details,
		Location:           item.Location,
	}
}
