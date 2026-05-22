package application

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"slices"
	"testing"
	"time"

	"bank-of-vivaldi/internal/domain"
)

type fakeStore struct {
	settings domain.AppSettings
	vaults   map[string]domain.Vault
	items    map[string]domain.Item
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		settings: domain.AppSettings{DefaultEncumbranceMode: domain.EncumbranceModeStandard},
		vaults:   map[string]domain.Vault{},
		items:    map[string]domain.Item{},
	}
}

func (f *fakeStore) GetAppSettings(context.Context) (domain.AppSettings, error) {
	return f.settings, nil
}

func (f *fakeStore) SetDefaultEncumbranceMode(_ context.Context, mode domain.EncumbranceMode) error {
	f.settings.DefaultEncumbranceMode = mode
	return nil
}

func (f *fakeStore) CreateVault(_ context.Context, vault domain.Vault) (domain.Vault, error) {
	f.vaults[vault.ID] = vault
	return vault, nil
}

func (f *fakeStore) UpdateVault(_ context.Context, vault domain.Vault) (domain.Vault, error) {
	f.vaults[vault.ID] = vault
	return vault, nil
}

func (f *fakeStore) GetVault(_ context.Context, id string) (domain.Vault, error) {
	vault, ok := f.vaults[id]
	if !ok {
		return domain.Vault{}, fmt.Errorf("vault not found")
	}
	return vault, nil
}

func (f *fakeStore) ListVaults(_ context.Context, includeArchived bool) ([]domain.Vault, error) {
	vaults := make([]domain.Vault, 0, len(f.vaults))
	for _, vault := range f.vaults {
		if !includeArchived && vault.Archived {
			continue
		}
		vaults = append(vaults, vault)
	}
	return vaults, nil
}

func (f *fakeStore) SavePurse(_ context.Context, vaultID string, purse domain.Purse) error {
	vault := f.vaults[vaultID]
	vault.Purse = purse
	f.vaults[vaultID] = vault
	return nil
}

func (f *fakeStore) CreateItem(_ context.Context, item domain.Item) (domain.Item, error) {
	f.items[item.ID] = item
	return item, nil
}

func (f *fakeStore) UpdateItem(_ context.Context, item domain.Item) (domain.Item, error) {
	f.items[item.ID] = item
	return item, nil
}

func (f *fakeStore) GetItem(_ context.Context, id string) (domain.Item, error) {
	item, ok := f.items[id]
	if !ok {
		return domain.Item{}, fmt.Errorf("item not found")
	}
	return item, nil
}

func (f *fakeStore) ListItems(context.Context) ([]domain.Item, error) {
	items := make([]domain.Item, 0, len(f.items))
	for _, item := range f.items {
		items = append(items, item)
	}
	return items, nil
}

func (f *fakeStore) DeleteItem(_ context.Context, id string) error {
	delete(f.items, id)
	return nil
}

func TestMoveItemFromCompendiumToVault(t *testing.T) {
	store := newFakeStore()
	store.vaults["vault-1"] = domain.Vault{
		ID:              "vault-1",
		CharacterName:   "Lyra",
		StrengthScore:   10,
		EncumbranceMode: domain.EncumbranceModeStandard,
	}
	store.items["item-1"] = domain.Item{
		ID:                 "item-1",
		Name:               "Rope",
		Slug:               "rope",
		Category:           "equipment",
		Rarity:             domain.RarityMundane,
		Quantity:           1,
		WeightHundredthsLB: 1000,
		BaseValueCP:        100,
		SourceKind:         domain.SourceKindManual,
		Location:           domain.ItemLocation{Kind: domain.LocationKindCompendiumRoot},
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	svc := NewService(store)
	item, err := svc.MoveItem(context.Background(), "item-1", domain.ItemLocation{
		Kind:         domain.LocationKindVaultRoot,
		OwnerVaultID: "vault-1",
	})
	if err != nil {
		t.Fatalf("move item: %v", err)
	}
	if item.Location.OwnerVaultID != "vault-1" || item.Location.Kind != domain.LocationKindVaultRoot {
		t.Fatalf("unexpected item location: %+v", item.Location)
	}
}

func TestMoveItemRejectsOverCapacityContainer(t *testing.T) {
	store := newFakeStore()
	store.vaults["vault-1"] = domain.Vault{
		ID:              "vault-1",
		CharacterName:   "Lyra",
		StrengthScore:   20,
		EncumbranceMode: domain.EncumbranceModeStandard,
	}
	store.items["bag-1"] = domain.Item{
		ID:          "bag-1",
		Name:        "Backpack",
		Slug:        "backpack",
		Category:    "container",
		Rarity:      domain.RarityMundane,
		Quantity:    1,
		IsContainer: true,
		Details:     domain.ItemDetails{Container: &domain.ContainerDetails{MaxWeightHundredthsLB: 3000}},
		Location:    domain.ItemLocation{Kind: domain.LocationKindVaultRoot, OwnerVaultID: "vault-1"},
		SourceKind:  domain.SourceKindManual,
	}
	store.items["bag-2"] = domain.Item{
		ID:          "bag-2",
		Name:        "Pouch",
		Slug:        "pouch",
		Category:    "container",
		Rarity:      domain.RarityMundane,
		Quantity:    1,
		IsContainer: true,
		Details:     domain.ItemDetails{Container: &domain.ContainerDetails{MaxWeightHundredthsLB: 100}},
		Location:    domain.ItemLocation{Kind: domain.LocationKindVaultRoot, OwnerVaultID: "vault-1"},
		SourceKind:  domain.SourceKindManual,
	}
	store.items["rock"] = domain.Item{
		ID:                 "rock",
		Name:               "Rock",
		Slug:               "rock",
		Category:           "equipment",
		Rarity:             domain.RarityMundane,
		Quantity:           1,
		WeightHundredthsLB: 200,
		BaseValueCP:        1,
		SourceKind:         domain.SourceKindManual,
		Location:           domain.ItemLocation{Kind: domain.LocationKindContainer, OwnerVaultID: "vault-1", ParentContainerItemID: "bag-1"},
	}

	svc := NewService(store)
	if _, err := svc.MoveItem(context.Background(), "rock", domain.ItemLocation{
		Kind:                  domain.LocationKindContainer,
		ParentContainerItemID: "bag-2",
	}); err == nil {
		t.Fatalf("expected container capacity rejection")
	}
}

func TestSplitAndMergeStack(t *testing.T) {
	store := newFakeStore()
	store.vaults["vault-1"] = domain.Vault{
		ID:              "vault-1",
		CharacterName:   "Lyra",
		StrengthScore:   20,
		EncumbranceMode: domain.EncumbranceModeStandard,
	}
	store.items["torch-1"] = domain.Item{
		ID:                 "torch-1",
		Name:               "Torch",
		Slug:               "torch",
		Category:           "equipment",
		Rarity:             domain.RarityMundane,
		Quantity:           5,
		WeightHundredthsLB: 100,
		BaseValueCP:        10,
		IsStackable:        true,
		SourceKind:         domain.SourceKindManual,
		Location:           domain.ItemLocation{Kind: domain.LocationKindVaultRoot, OwnerVaultID: "vault-1"},
	}

	svc := NewService(store)
	if err := svc.SplitStack(context.Background(), "torch-1", 2); err != nil {
		t.Fatalf("split stack: %v", err)
	}

	items, _ := store.ListItems(context.Background())
	if len(items) != 2 {
		t.Fatalf("expected 2 items after split, got %d", len(items))
	}

	var splitID string
	for _, item := range items {
		if item.ID != "torch-1" {
			splitID = item.ID
			break
		}
	}
	if splitID == "" {
		t.Fatalf("split item not found")
	}

	if err := svc.MergeStacks(context.Background(), splitID, "torch-1"); err != nil {
		t.Fatalf("merge stacks: %v", err)
	}

	items, _ = store.ListItems(context.Background())
	if len(items) != 1 {
		t.Fatalf("expected 1 item after merge, got %d", len(items))
	}
	if items[0].Quantity != 5 {
		t.Fatalf("expected merged quantity 5, got %d", items[0].Quantity)
	}
}

func TestSearchAcrossNestedStorage(t *testing.T) {
	store := newFakeStore()
	store.vaults["vault-1"] = domain.Vault{
		ID:              "vault-1",
		CharacterName:   "Lyra",
		StrengthScore:   12,
		EncumbranceMode: domain.EncumbranceModeStandard,
	}
	store.items["backpack"] = domain.Item{
		ID:          "backpack",
		Name:        "Backpack",
		Slug:        "backpack",
		Category:    "container",
		Rarity:      domain.RarityMundane,
		Quantity:    1,
		IsContainer: true,
		Details:     domain.ItemDetails{Container: &domain.ContainerDetails{MaxWeightHundredthsLB: 3000}},
		SourceKind:  domain.SourceKindManual,
		Location:    domain.ItemLocation{Kind: domain.LocationKindVaultRoot, OwnerVaultID: "vault-1"},
	}
	store.items["gem"] = domain.Item{
		ID:                 "gem",
		Name:               "Gemstone",
		Slug:               "gemstone",
		Category:           "treasure",
		Rarity:             domain.RarityRare,
		Quantity:           1,
		WeightHundredthsLB: 10,
		BaseValueCP:        5000,
		SourceKind:         domain.SourceKindManual,
		Location:           domain.ItemLocation{Kind: domain.LocationKindContainer, OwnerVaultID: "vault-1", ParentContainerItemID: "backpack"},
	}

	svc := NewService(store)
	results, _, err := svc.SearchInventory(context.Background(), SearchFilters{
		Query:     "gem",
		Container: "backpack",
	})
	if err != nil {
		t.Fatalf("search inventory: %v", err)
	}
	if len(results) != 1 || results[0].Item.ID != "gem" {
		t.Fatalf("unexpected search results: %+v", results)
	}
}

func TestCommitBulkCreatesItems(t *testing.T) {
	store := newFakeStore()
	store.vaults["vault-1"] = domain.Vault{
		ID:              "vault-1",
		CharacterName:   "Lyra",
		StrengthScore:   20,
		EncumbranceMode: domain.EncumbranceModeStandard,
	}

	rows := []domain.BulkPreviewRow{
		{
			LineNumber:         1,
			Name:               "Lantern",
			Quantity:           1,
			Category:           "equipment",
			Rarity:             domain.RarityMundane,
			WeightHundredthsLB: 100,
			BaseValueCP:        500,
		},
	}
	bytes, _ := json.Marshal(rows)
	encoded := base64.StdEncoding.EncodeToString(bytes)

	svc := NewService(store)
	if err := svc.CommitBulk(context.Background(), BulkCommitInput{
		EncodedRows:  encoded,
		LocationKind: domain.LocationKindVaultRoot,
		VaultID:      "vault-1",
	}); err != nil {
		t.Fatalf("commit bulk: %v", err)
	}

	items, _ := store.ListItems(context.Background())
	if len(items) != 1 {
		t.Fatalf("expected 1 item after bulk commit, got %d", len(items))
	}
	if !slices.ContainsFunc(items, func(item domain.Item) bool { return item.Name == "Lantern" }) {
		t.Fatalf("expected Lantern item to exist")
	}
}
