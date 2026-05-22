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
	Settings             domain.AppSettings
	Vaults               []VaultSummary
	CompendiumWeight     int
	CompendiumValueCP    int
	CompendiumRootItems  []ItemNode
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
	EncodedRows            string
	LocationKind           domain.LocationKind
	VaultID                string
	ParentContainerItemID  string
	IsStackable            bool
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
		Settings:         settings,
		Vaults:           summarizeVaults(vaults, items),
		CompendiumWeight: domain.ComputeCompendiumWeightHundredths(items),
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

func (s *Service) ListCompendium(ctx context.Context) ([]ItemNode, domain.AppSettings, error) {
	settings, err := s.store.GetAppSettings(ctx)
	if err != nil {
		return nil, domain.AppSettings{}, err
	}

	items, err := s.store.ListItems(ctx)
	if err != nil {
		return nil, domain.AppSettings{}, err
	}

	return buildNodes(filterRootItems(items, domain.LocationKindCompendiumRoot, ""), items), settings, nil
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
	if err := s.validateSimulation(simulated, item); err != nil {
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

func (s *Service) PreviewBulk(text string, defaults domain.BulkDefaults) domain.BulkPreview {
	return domain.ParseBulkItemInput(text, defaults)
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

	for _, row := range rows {
		_, err := s.SaveItem(ctx, SaveItemInput{
			Name:               row.Name,
			Category:           row.Category,
			Rarity:             row.Rarity,
			WeightHundredthsLB: row.WeightHundredthsLB,
			BaseValueCP:        row.BaseValueCP,
			Quantity:           row.Quantity,
			IsStackable:        input.IsStackable,
			SourceKind:         domain.SourceKindManual,
			Location: domain.ItemLocation{
				Kind:                  input.LocationKind,
				OwnerVaultID:          input.VaultID,
				ParentContainerItemID: input.ParentContainerItemID,
			},
		})
		if err != nil {
			return err
		}
	}

	return nil
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
			return fmt.Errorf("vault destination requires a vault")
		}
		item.Location.ParentContainerItemID = ""
	case domain.LocationKindContainer:
		if item.Location.ParentContainerItemID == "" {
			return fmt.Errorf("container destination requires a target container")
		}
		parent, ok := findItem(allItems, item.Location.ParentContainerItemID)
		if !ok {
			return fmt.Errorf("target container not found")
		}
		if !parent.IsContainer {
			return fmt.Errorf("target item is not a container")
		}
		item.Location.OwnerVaultID = parent.Location.OwnerVaultID
	default:
		return fmt.Errorf("unsupported location kind")
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

func (s *Service) validateSimulation(items []domain.Item, target domain.Item) error {
	if target.Location.Kind == domain.LocationKindContainer {
		byID, children := domain.BuildItemIndexes(items)
		container, ok := byID[target.Location.ParentContainerItemID]
		if !ok {
			return fmt.Errorf("target container not found")
		}
		if err := domain.ValidateContainerCapacity(container, domain.ComputeContainedWeightHundredths(container.ID, byID, children)); err != nil {
			return err
		}
	}

	if target.IsContainer {
		byID, children := domain.BuildItemIndexes(items)
		container, ok := byID[target.ID]
		if ok {
			if err := domain.ValidateContainerCapacity(container, domain.ComputeContainedWeightHundredths(container.ID, byID, children)); err != nil {
				return err
			}
		}
	}

	if target.Location.OwnerVaultID != "" {
		vault, err := s.store.GetVault(context.Background(), target.Location.OwnerVaultID)
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
	switch sortBy {
	case "weight":
		sort.Slice(results, func(i, j int) bool {
			return results[i].Item.TotalWeightHundredthsLB() < results[j].Item.TotalWeightHundredthsLB()
		})
	case "value":
		sort.Slice(results, func(i, j int) bool {
			return results[i].Item.TotalValueCP() < results[j].Item.TotalValueCP()
		})
	case "category":
		sort.Slice(results, func(i, j int) bool {
			return results[i].Item.Category < results[j].Item.Category
		})
	case "rarity":
		sort.Slice(results, func(i, j int) bool {
			return string(results[i].Item.Rarity) < string(results[j].Item.Rarity)
		})
	default:
		sort.Slice(results, func(i, j int) bool {
			return strings.ToLower(results[i].Item.Name) < strings.ToLower(results[j].Item.Name)
		})
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
