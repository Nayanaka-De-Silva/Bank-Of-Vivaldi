package application

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"testing"
	"time"

	"bank-of-vivaldi/internal/domain"
)

type fakeStore struct {
	settings        domain.AppSettings
	vaults          map[string]domain.Vault
	items           map[string]domain.Item
	createOrder     []string // IDs in CreateItem call order
	failCreateAfter int      // if > 0, fail CreateItem after this many successes
	createCount     int      // number of successful CreateItem calls so far
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
	if f.failCreateAfter > 0 && f.createCount >= f.failCreateAfter {
		return domain.Item{}, fmt.Errorf("simulated create failure")
	}
	f.items[item.ID] = item
	f.createOrder = append(f.createOrder, item.ID)
	f.createCount++
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

// --- PreviewBulk name-resolution tests ---

func TestPreviewBulkResolvesVaultByName(t *testing.T) {
	store := newFakeStore()
	store.vaults["vault-1"] = domain.Vault{
		ID:            "vault-1",
		CharacterName: "Lyra",
		StrengthScore: 20,
	}
	svc := NewService(store)

	preview, err := svc.PreviewBulk(context.Background(), "Rope | equipment | mundane | 10 | 100 | vault=Lyra", domain.BulkDefaults{})
	if err != nil {
		t.Fatalf("PreviewBulk error: %v", err)
	}
	if len(preview.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(preview.Rows))
	}
	row := preview.Rows[0]
	if len(row.Errors) > 0 {
		t.Fatalf("unexpected row errors: %v", row.Errors)
	}
	if row.ResolvedVaultID != "vault-1" {
		t.Errorf("ResolvedVaultID = %q, want \"vault-1\"", row.ResolvedVaultID)
	}
}

func TestPreviewBulkRejectsUnknownVaultName(t *testing.T) {
	store := newFakeStore()
	svc := NewService(store)

	preview, err := svc.PreviewBulk(context.Background(), "Rope | equipment | mundane | 10 | 100 | vault=Ghost", domain.BulkDefaults{})
	if err != nil {
		t.Fatalf("PreviewBulk error: %v", err)
	}
	if len(preview.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(preview.Rows))
	}
	if len(preview.Rows[0].Errors) == 0 {
		t.Error("expected row error for unknown vault name, got none")
	}
}

func TestPreviewBulkResolvesContainerByName(t *testing.T) {
	store := newFakeStore()
	store.items["backpack-1"] = domain.Item{
		ID:          "backpack-1",
		Name:        "Backpack",
		Slug:        "backpack",
		Category:    "container",
		Rarity:      domain.RarityMundane,
		Quantity:    1,
		IsContainer: true,
		SourceKind:  domain.SourceKindManual,
		Location:    domain.ItemLocation{Kind: domain.LocationKindCompendiumRoot},
	}
	svc := NewService(store)

	preview, err := svc.PreviewBulk(context.Background(), "Rope | equipment | mundane | 10 | 100 | parent=Backpack", domain.BulkDefaults{})
	if err != nil {
		t.Fatalf("PreviewBulk error: %v", err)
	}
	if len(preview.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(preview.Rows))
	}
	row := preview.Rows[0]
	if len(row.Errors) > 0 {
		t.Fatalf("unexpected row errors: %v", row.Errors)
	}
	if row.ResolvedContainerID != "backpack-1" {
		t.Errorf("ResolvedContainerID = %q, want \"backpack-1\"", row.ResolvedContainerID)
	}
}

func TestPreviewBulkRejectsNonContainerAsParent(t *testing.T) {
	store := newFakeStore()
	store.items["sword-1"] = domain.Item{
		ID:          "sword-1",
		Name:        "Sword",
		Slug:        "sword",
		Category:    "weapon",
		Rarity:      domain.RarityMundane,
		Quantity:    1,
		IsContainer: false,
		SourceKind:  domain.SourceKindManual,
		Location:    domain.ItemLocation{Kind: domain.LocationKindCompendiumRoot},
	}
	svc := NewService(store)

	preview, err := svc.PreviewBulk(context.Background(), "Rope | equipment | mundane | 10 | 100 | parent=Sword", domain.BulkDefaults{})
	if err != nil {
		t.Fatalf("PreviewBulk error: %v", err)
	}
	if len(preview.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(preview.Rows))
	}
	if len(preview.Rows[0].Errors) == 0 {
		t.Error("expected row error for non-container parent, got none")
	}
}

// --- CommitBulk per-row override and batch fallback tests ---

func TestCommitBulkRowOverrideBeatsDefault(t *testing.T) {
	// Row says stackable=false; batch says IsStackable=true — row wins.
	store := newFakeStore()
	store.vaults["vault-1"] = domain.Vault{ID: "vault-1", CharacterName: "Lyra", StrengthScore: 20}

	f := false
	rows := []domain.BulkPreviewRow{{
		LineNumber:         1,
		Name:               "Rope",
		Quantity:           1,
		Category:           "equipment",
		Rarity:             domain.RarityMundane,
		WeightHundredthsLB: 1000,
		BaseValueCP:        100,
		IsStackable:        &f,
	}}
	bytes, _ := json.Marshal(rows)
	encoded := base64.StdEncoding.EncodeToString(bytes)

	svc := NewService(store)
	if err := svc.CommitBulk(context.Background(), BulkCommitInput{
		EncodedRows:  encoded,
		LocationKind: domain.LocationKindVaultRoot,
		VaultID:      "vault-1",
		IsStackable:  true, // batch default = true, but row override = false
	}); err != nil {
		t.Fatalf("commit bulk: %v", err)
	}

	items, _ := store.ListItems(context.Background())
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	for _, item := range items {
		if item.IsStackable {
			t.Errorf("IsStackable = true, want false (row override should win)")
		}
	}
}

func TestCommitBulkBatchFallback(t *testing.T) {
	// Row has no IsStackable pointer — should fall back to batch default.
	store := newFakeStore()
	store.vaults["vault-1"] = domain.Vault{ID: "vault-1", CharacterName: "Lyra", StrengthScore: 20}

	rows := []domain.BulkPreviewRow{{
		LineNumber:         1,
		Name:               "Rope",
		Quantity:           1,
		Category:           "equipment",
		Rarity:             domain.RarityMundane,
		WeightHundredthsLB: 1000,
		BaseValueCP:        100,
		// IsStackable is nil (no row override)
	}}
	bytes, _ := json.Marshal(rows)
	encoded := base64.StdEncoding.EncodeToString(bytes)

	svc := NewService(store)
	if err := svc.CommitBulk(context.Background(), BulkCommitInput{
		EncodedRows:  encoded,
		LocationKind: domain.LocationKindVaultRoot,
		VaultID:      "vault-1",
		IsStackable:  true,
	}); err != nil {
		t.Fatalf("commit bulk: %v", err)
	}

	items, _ := store.ListItems(context.Background())
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	for _, item := range items {
		if !item.IsStackable {
			t.Errorf("IsStackable = false, want true (batch fallback)")
		}
	}
}

func TestCommitBulkResolvedLocationOverride(t *testing.T) {
	// Row has ResolvedVaultID — it should override the batch VaultID.
	store := newFakeStore()
	store.vaults["vault-1"] = domain.Vault{ID: "vault-1", CharacterName: "Lyra", StrengthScore: 20}
	store.vaults["vault-2"] = domain.Vault{ID: "vault-2", CharacterName: "Zara", StrengthScore: 20}

	rows := []domain.BulkPreviewRow{{
		LineNumber:      1,
		Name:            "Dagger",
		Quantity:        1,
		Category:        "weapon",
		Rarity:          domain.RarityMundane,
		ResolvedVaultID: "vault-2", // row-level override
	}}
	bytes, _ := json.Marshal(rows)
	encoded := base64.StdEncoding.EncodeToString(bytes)

	svc := NewService(store)
	if err := svc.CommitBulk(context.Background(), BulkCommitInput{
		EncodedRows:  encoded,
		LocationKind: domain.LocationKindVaultRoot,
		VaultID:      "vault-1", // batch default
	}); err != nil {
		t.Fatalf("commit bulk: %v", err)
	}

	items, _ := store.ListItems(context.Background())
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	for _, item := range items {
		if item.Location.OwnerVaultID != "vault-2" {
			t.Errorf("OwnerVaultID = %q, want \"vault-2\" (resolved override)", item.Location.OwnerVaultID)
		}
	}
}

func TestCommitBulkWeaponDetailsSurviveRoundTrip(t *testing.T) {
	store := newFakeStore()
	store.vaults["vault-1"] = domain.Vault{ID: "vault-1", CharacterName: "Lyra", StrengthScore: 20}

	rows := []domain.BulkPreviewRow{{
		LineNumber: 1,
		Name:       "Dagger",
		Quantity:   1,
		Category:   "weapon",
		Rarity:     domain.RarityMundane,
		Details: domain.ItemDetails{
			Weapon: &domain.WeaponDetails{
				DamageDice: "1d4",
				DamageType: "piercing",
				Properties: []string{"finesse", "light", "thrown"},
			},
		},
	}}
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
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	for _, item := range items {
		if item.Details.Weapon == nil {
			t.Fatal("saved item has no WeaponDetails")
		}
		if item.Details.Weapon.DamageDice != "1d4" {
			t.Errorf("DamageDice = %q, want \"1d4\"", item.Details.Weapon.DamageDice)
		}
		if len(item.Details.Weapon.Properties) != 3 {
			t.Errorf("Properties len = %d, want 3", len(item.Details.Weapon.Properties))
		}
	}
}

// threeLevelFixture returns a bag → pouch → coins hierarchy rooted at the compendium.
// bag has MaxWeightHundredthsLB=5000; pouch has 2000; coins weigh 200 each.
func threeLevelFixture() (bag, pouch, coins domain.Item) {
	srcTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	bag = domain.Item{
		ID: "bag", Name: "Bag", Slug: "bag", Category: "container",
		Rarity: domain.RarityMundane, Quantity: 1,
		WeightHundredthsLB: 100,
		IsContainer:        true,
		SourceKind:         domain.SourceKindManual,
		Details:            domain.ItemDetails{Container: &domain.ContainerDetails{MaxWeightHundredthsLB: 5000}},
		Location:           domain.ItemLocation{Kind: domain.LocationKindCompendiumRoot},
		CreatedAt:          srcTime,
		UpdatedAt:          srcTime,
	}
	pouch = domain.Item{
		ID: "pouch", Name: "Pouch", Slug: "pouch", Category: "container",
		Rarity: domain.RarityMundane, Quantity: 1,
		WeightHundredthsLB: 100,
		IsContainer:        true,
		SourceKind:         domain.SourceKindManual,
		Details:            domain.ItemDetails{Container: &domain.ContainerDetails{MaxWeightHundredthsLB: 2000}},
		Location:           domain.ItemLocation{Kind: domain.LocationKindContainer, ParentContainerItemID: "bag"},
		CreatedAt:          srcTime,
		UpdatedAt:          srcTime,
	}
	coins = domain.Item{
		ID: "coins", Name: "Coins", Slug: "coins", Category: "treasure",
		Rarity: domain.RarityMundane, Quantity: 1,
		WeightHundredthsLB: 200,
		SourceKind:         domain.SourceKindManual,
		Location:           domain.ItemLocation{Kind: domain.LocationKindContainer, ParentContainerItemID: "pouch"},
		CreatedAt:          srcTime,
		UpdatedAt:          srcTime,
	}
	return
}

func TestCopyItemDuplicatesLeafIntoVault(t *testing.T) {
	store := newFakeStore()
	store.vaults["vault-1"] = domain.Vault{
		ID: "vault-1", CharacterName: "Lyra",
		StrengthScore: 20, EncumbranceMode: domain.EncumbranceModeStandard,
	}
	srcTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	store.items["leaf-1"] = domain.Item{
		ID: "leaf-1", Name: "Gem", Slug: "gem", Category: "treasure",
		Rarity: domain.RarityMundane, Quantity: 1,
		WeightHundredthsLB: 10, BaseValueCP: 500,
		SourceKind: domain.SourceKindManual,
		Location:   domain.ItemLocation{Kind: domain.LocationKindCompendiumRoot},
		CreatedAt:  srcTime, UpdatedAt: srcTime,
	}

	svc := NewService(store)
	got, err := svc.CopyItem(context.Background(), "leaf-1", domain.ItemLocation{
		Kind: domain.LocationKindVaultRoot, OwnerVaultID: "vault-1",
	})
	if err != nil {
		t.Fatalf("CopyItem: %v", err)
	}
	if got.ID == "leaf-1" {
		t.Fatalf("clone should have fresh ID, got same as source")
	}
	if got.Name != "Gem" {
		t.Fatalf("clone name mismatch: got %q", got.Name)
	}
	if got.Location.OwnerVaultID != "vault-1" || got.Location.Kind != domain.LocationKindVaultRoot {
		t.Fatalf("unexpected clone location: %+v", got.Location)
	}

	items, _ := store.ListItems(context.Background())
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	src := store.items["leaf-1"]
	if src.Location.Kind != domain.LocationKindCompendiumRoot {
		t.Fatalf("source should still be in compendium, got %+v", src.Location)
	}
}

func TestCopyItemAssignsFreshIDsToEveryDescendant(t *testing.T) {
	store := newFakeStore()
	store.vaults["vault-1"] = domain.Vault{
		ID: "vault-1", CharacterName: "Lyra",
		StrengthScore: 20, EncumbranceMode: domain.EncumbranceModeStandard,
	}
	bag, pouch, coins := threeLevelFixture()
	store.items[bag.ID] = bag
	store.items[pouch.ID] = pouch
	store.items[coins.ID] = coins

	svc := NewService(store)
	_, err := svc.CopyItem(context.Background(), "bag", domain.ItemLocation{
		Kind: domain.LocationKindVaultRoot, OwnerVaultID: "vault-1",
	})
	if err != nil {
		t.Fatalf("CopyItem: %v", err)
	}

	items, _ := store.ListItems(context.Background())
	if len(items) != 6 {
		t.Fatalf("expected 6 items, got %d", len(items))
	}

	sourceIDs := map[string]bool{"bag": true, "pouch": true, "coins": true}
	cloneIDs := map[string]bool{}
	for _, item := range items {
		if sourceIDs[item.ID] {
			continue
		}
		if cloneIDs[item.ID] {
			t.Fatalf("duplicate clone ID: %q", item.ID)
		}
		cloneIDs[item.ID] = true
		if sourceIDs[item.ID] {
			t.Fatalf("clone ID %q collides with a source ID", item.ID)
		}
	}
	if len(cloneIDs) != 3 {
		t.Fatalf("expected 3 distinct clone IDs, got %d", len(cloneIDs))
	}
}

func TestCopyItemReparentsClonedChildrenToClonedParent(t *testing.T) {
	store := newFakeStore()
	store.vaults["vault-1"] = domain.Vault{
		ID: "vault-1", CharacterName: "Lyra",
		StrengthScore: 20, EncumbranceMode: domain.EncumbranceModeStandard,
	}
	bag, pouch, coins := threeLevelFixture()
	store.items[bag.ID] = bag
	store.items[pouch.ID] = pouch
	store.items[coins.ID] = coins

	svc := NewService(store)
	clonedBag, err := svc.CopyItem(context.Background(), "bag", domain.ItemLocation{
		Kind: domain.LocationKindVaultRoot, OwnerVaultID: "vault-1",
	})
	if err != nil {
		t.Fatalf("CopyItem: %v", err)
	}

	items, _ := store.ListItems(context.Background())
	var clonedPouch domain.Item
	for _, item := range items {
		if item.Location.ParentContainerItemID == clonedBag.ID {
			clonedPouch = item
		}
	}
	if clonedPouch.ID == "" {
		t.Fatalf("cloned pouch not found (no item with parent == cloned bag %q)", clonedBag.ID)
	}

	var clonedCoins domain.Item
	for _, item := range items {
		if item.Location.ParentContainerItemID == clonedPouch.ID && item.ID != clonedPouch.ID {
			clonedCoins = item
		}
	}
	if clonedCoins.ID == "" {
		t.Fatalf("cloned coins not found (no item with parent == cloned pouch %q)", clonedPouch.ID)
	}

	if store.items["pouch"].Location.ParentContainerItemID != "bag" {
		t.Fatalf("original pouch parent mutated: %q", store.items["pouch"].Location.ParentContainerItemID)
	}
	if store.items["coins"].Location.ParentContainerItemID != "pouch" {
		t.Fatalf("original coins parent mutated: %q", store.items["coins"].Location.ParentContainerItemID)
	}
}

func TestCopyItemPropagatesOwnerVaultToDescendants(t *testing.T) {
	store := newFakeStore()
	store.vaults["vault-1"] = domain.Vault{
		ID: "vault-1", CharacterName: "Lyra",
		StrengthScore: 20, EncumbranceMode: domain.EncumbranceModeStandard,
	}
	bag, pouch, coins := threeLevelFixture()
	store.items[bag.ID] = bag
	store.items[pouch.ID] = pouch
	store.items[coins.ID] = coins

	svc := NewService(store)
	_, err := svc.CopyItem(context.Background(), "bag", domain.ItemLocation{
		Kind: domain.LocationKindVaultRoot, OwnerVaultID: "vault-1",
	})
	if err != nil {
		t.Fatalf("CopyItem: %v", err)
	}

	sourceIDs := map[string]bool{"bag": true, "pouch": true, "coins": true}
	items, _ := store.ListItems(context.Background())
	for _, item := range items {
		if sourceIDs[item.ID] {
			continue
		}
		if item.Location.OwnerVaultID != "vault-1" {
			t.Fatalf("clone %q (%s) OwnerVaultID=%q, want vault-1", item.ID, item.Name, item.Location.OwnerVaultID)
		}
	}
}

func TestCopyItemRejectsOverCapacityVaultLeavingNoPartialState(t *testing.T) {
	store := newFakeStore()
	// StrengthScore=1 → capacity = 1*15*100 = 1500 hundredths
	store.vaults["vault-1"] = domain.Vault{
		ID: "vault-1", CharacterName: "Lyra",
		StrengthScore: 1, EncumbranceMode: domain.EncumbranceModeStandard,
	}
	// Pre-existing item using 1000 hundredths
	store.items["sword"] = domain.Item{
		ID: "sword", Name: "Sword", Slug: "sword", Category: "weapon",
		Rarity: domain.RarityMundane, Quantity: 1,
		WeightHundredthsLB: 1000, SourceKind: domain.SourceKindManual,
		Location: domain.ItemLocation{Kind: domain.LocationKindVaultRoot, OwnerVaultID: "vault-1"},
	}
	// bag(100) + pouch(100) + coins(400) = 600; 1000+600=1600 > 1500
	bag, pouch, coins := threeLevelFixture()
	coins.WeightHundredthsLB = 400
	store.items[bag.ID] = bag
	store.items[pouch.ID] = pouch
	store.items[coins.ID] = coins

	svc := NewService(store)
	_, err := svc.CopyItem(context.Background(), "bag", domain.ItemLocation{
		Kind: domain.LocationKindVaultRoot, OwnerVaultID: "vault-1",
	})
	if err == nil {
		t.Fatalf("expected over-capacity vault rejection")
	}

	items, _ := store.ListItems(context.Background())
	if len(items) != 4 { // sword + bag + pouch + coins only
		t.Fatalf("expected 4 items (no partial state), got %d", len(items))
	}
}

func TestCopyItemRejectsOverCapacityContainerCountingWholeSubtree(t *testing.T) {
	store := newFakeStore()
	store.vaults["vault-1"] = domain.Vault{
		ID: "vault-1", CharacterName: "Lyra",
		StrengthScore: 20, EncumbranceMode: domain.EncumbranceModeStandard,
	}
	// Destination container: capacity 300 hundredths
	store.items["dest"] = domain.Item{
		ID: "dest", Name: "Chest", Slug: "chest", Category: "container",
		Rarity: domain.RarityMundane, Quantity: 1,
		IsContainer: true,
		Details:     domain.ItemDetails{Container: &domain.ContainerDetails{MaxWeightHundredthsLB: 300}},
		SourceKind:  domain.SourceKindManual,
		Location:    domain.ItemLocation{Kind: domain.LocationKindVaultRoot, OwnerVaultID: "vault-1"},
	}
	// bag(100) + pouch(100) + coins(200) = 400 > 300
	bag, pouch, coins := threeLevelFixture()
	store.items[bag.ID] = bag
	store.items[pouch.ID] = pouch
	store.items[coins.ID] = coins

	svc := NewService(store)
	_, err := svc.CopyItem(context.Background(), "bag", domain.ItemLocation{
		Kind:                  domain.LocationKindContainer,
		ParentContainerItemID: "dest",
	})
	if err == nil {
		t.Fatalf("expected over-capacity container rejection")
	}

	items, _ := store.ListItems(context.Background())
	if len(items) != 4 { // dest + bag + pouch + coins
		t.Fatalf("expected 4 items (no clones), got %d", len(items))
	}
}

func TestCopyItemDeepCopiesContainerDetails(t *testing.T) {
	store := newFakeStore()
	store.vaults["vault-1"] = domain.Vault{
		ID: "vault-1", CharacterName: "Lyra",
		StrengthScore: 20, EncumbranceMode: domain.EncumbranceModeStandard,
	}
	store.items["bag-src"] = domain.Item{
		ID: "bag-src", Name: "Bag", Slug: "bag", Category: "container",
		Rarity: domain.RarityMundane, Quantity: 1,
		IsContainer: true,
		Details:     domain.ItemDetails{Container: &domain.ContainerDetails{MaxWeightHundredthsLB: 500}},
		SourceKind:  domain.SourceKindManual,
		Location:    domain.ItemLocation{Kind: domain.LocationKindCompendiumRoot},
	}

	svc := NewService(store)
	got, err := svc.CopyItem(context.Background(), "bag-src", domain.ItemLocation{
		Kind: domain.LocationKindVaultRoot, OwnerVaultID: "vault-1",
	})
	if err != nil {
		t.Fatalf("CopyItem: %v", err)
	}

	// Mutate clone's Container through the store
	cloneInStore := store.items[got.ID]
	cloneInStore.Details.Container.MaxWeightHundredthsLB = 9999
	store.items[got.ID] = cloneInStore

	// Source must be unaffected
	srcInStore := store.items["bag-src"]
	if srcInStore.Details.Container.MaxWeightHundredthsLB != 500 {
		t.Fatalf("source Details.Container mutated: got %d, want 500",
			srcInStore.Details.Container.MaxWeightHundredthsLB)
	}
}

func TestCopyItemClearsEquippedFlag(t *testing.T) {
	store := newFakeStore()
	store.vaults["vault-1"] = domain.Vault{
		ID: "vault-1", CharacterName: "Lyra",
		StrengthScore: 20, EncumbranceMode: domain.EncumbranceModeStandard,
	}
	bag, pouch, _ := threeLevelFixture()
	bag.IsEquipped = true
	pouch.IsEquipped = true
	store.items[bag.ID] = bag
	store.items[pouch.ID] = pouch

	svc := NewService(store)
	clonedBag, err := svc.CopyItem(context.Background(), "bag", domain.ItemLocation{
		Kind: domain.LocationKindVaultRoot, OwnerVaultID: "vault-1",
	})
	if err != nil {
		t.Fatalf("CopyItem: %v", err)
	}
	if clonedBag.IsEquipped {
		t.Fatalf("cloned bag root should not be equipped")
	}

	sourceIDs := map[string]bool{"bag": true, "pouch": true}
	items, _ := store.ListItems(context.Background())
	for _, item := range items {
		if sourceIDs[item.ID] {
			continue
		}
		if item.IsEquipped {
			t.Fatalf("clone %q (%s) IsEquipped=true, want false", item.ID, item.Name)
		}
	}
}

func TestCopyItemStampsFreshTimestamps(t *testing.T) {
	store := newFakeStore()
	store.vaults["vault-1"] = domain.Vault{
		ID: "vault-1", CharacterName: "Lyra",
		StrengthScore: 20, EncumbranceMode: domain.EncumbranceModeStandard,
	}
	bag, pouch, coins := threeLevelFixture()
	store.items[bag.ID] = bag
	store.items[pouch.ID] = pouch
	store.items[coins.ID] = coins

	fixedNow := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	svc := NewService(store)
	svc.now = func() time.Time { return fixedNow }

	_, err := svc.CopyItem(context.Background(), "bag", domain.ItemLocation{
		Kind: domain.LocationKindVaultRoot, OwnerVaultID: "vault-1",
	})
	if err != nil {
		t.Fatalf("CopyItem: %v", err)
	}

	sourceIDs := map[string]bool{"bag": true, "pouch": true, "coins": true}
	items, _ := store.ListItems(context.Background())
	for _, item := range items {
		if sourceIDs[item.ID] {
			continue
		}
		if !item.CreatedAt.Equal(fixedNow) {
			t.Fatalf("clone %q CreatedAt=%v, want %v", item.ID, item.CreatedAt, fixedNow)
		}
		if !item.UpdatedAt.Equal(fixedNow) {
			t.Fatalf("clone %q UpdatedAt=%v, want %v", item.ID, item.UpdatedAt, fixedNow)
		}
		if item.CreatedAt.Equal(bag.CreatedAt) {
			t.Fatalf("clone %q timestamp not fresh (same as source)", item.ID)
		}
	}
}

func TestCopyItemIntoOwnDescendantIsAllowed(t *testing.T) {
	// Copying a container into its own descendant is allowed: the clone gets
	// fresh IDs and forms a disjoint subtree, so no actual cycle exists.
	store := newFakeStore()
	bag, pouch, _ := threeLevelFixture()
	store.items[bag.ID] = bag
	store.items[pouch.ID] = pouch

	svc := NewService(store)
	_, err := svc.CopyItem(context.Background(), "bag", domain.ItemLocation{
		Kind:                  domain.LocationKindContainer,
		ParentContainerItemID: "pouch",
	})
	if err != nil {
		t.Fatalf("copy-into-own-descendant should be allowed: %v", err)
	}
}

func TestCopyItemLeavesSourceSubtreeUnchanged(t *testing.T) {
	store := newFakeStore()
	store.vaults["vault-1"] = domain.Vault{
		ID: "vault-1", CharacterName: "Lyra",
		StrengthScore: 20, EncumbranceMode: domain.EncumbranceModeStandard,
	}
	bag, pouch, coins := threeLevelFixture()
	store.items[bag.ID] = bag
	store.items[pouch.ID] = pouch
	store.items[coins.ID] = coins

	svc := NewService(store)
	_, err := svc.CopyItem(context.Background(), "bag", domain.ItemLocation{
		Kind: domain.LocationKindVaultRoot, OwnerVaultID: "vault-1",
	})
	if err != nil {
		t.Fatalf("CopyItem: %v", err)
	}

	if !reflect.DeepEqual(store.items["bag"], bag) {
		t.Fatalf("source bag mutated after copy: %+v", store.items["bag"])
	}
	if !reflect.DeepEqual(store.items["pouch"], pouch) {
		t.Fatalf("source pouch mutated after copy: %+v", store.items["pouch"])
	}
	if !reflect.DeepEqual(store.items["coins"], coins) {
		t.Fatalf("source coins mutated after copy: %+v", store.items["coins"])
	}
}

func TestCopyItemRequiresValidDestination(t *testing.T) {
	store := newFakeStore()
	store.vaults["vault-1"] = domain.Vault{
		ID: "vault-1", CharacterName: "Lyra",
		StrengthScore: 20, EncumbranceMode: domain.EncumbranceModeStandard,
	}
	store.items["leaf"] = domain.Item{
		ID: "leaf", Name: "Gem", Slug: "gem", Category: "treasure",
		Rarity: domain.RarityMundane, Quantity: 1,
		WeightHundredthsLB: 10, SourceKind: domain.SourceKindManual,
		Location: domain.ItemLocation{Kind: domain.LocationKindCompendiumRoot},
	}
	store.items["rock"] = domain.Item{
		ID: "rock", Name: "Rock", Slug: "rock", Category: "equipment",
		Rarity: domain.RarityMundane, Quantity: 1,
		WeightHundredthsLB: 100, SourceKind: domain.SourceKindManual,
		Location: domain.ItemLocation{Kind: domain.LocationKindVaultRoot, OwnerVaultID: "vault-1"},
	}

	svc := NewService(store)
	cases := []struct {
		name     string
		itemID   string
		location domain.ItemLocation
	}{
		{"unknown item ID", "does-not-exist", domain.ItemLocation{Kind: domain.LocationKindVaultRoot, OwnerVaultID: "vault-1"}},
		{"vault kind with empty vault ID", "leaf", domain.ItemLocation{Kind: domain.LocationKindVaultRoot}},
		{"container kind with empty container ID", "leaf", domain.ItemLocation{Kind: domain.LocationKindContainer}},
		{"container ID pointing at non-container", "leaf", domain.ItemLocation{Kind: domain.LocationKindContainer, ParentContainerItemID: "rock"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.CopyItem(context.Background(), tc.itemID, tc.location)
			if err == nil {
				t.Fatalf("expected error for %q", tc.name)
			}
		})
	}
}

func TestCopyItemCreatesParentsBeforeChildren(t *testing.T) {
	store := newFakeStore()
	store.vaults["vault-1"] = domain.Vault{
		ID: "vault-1", CharacterName: "Lyra",
		StrengthScore: 20, EncumbranceMode: domain.EncumbranceModeStandard,
	}
	bag, pouch, coins := threeLevelFixture()
	store.items[bag.ID] = bag
	store.items[pouch.ID] = pouch
	store.items[coins.ID] = coins

	svc := NewService(store)
	_, err := svc.CopyItem(context.Background(), "bag", domain.ItemLocation{
		Kind: domain.LocationKindVaultRoot, OwnerVaultID: "vault-1",
	})
	if err != nil {
		t.Fatalf("CopyItem: %v", err)
	}

	// Build position map: ID → index in createOrder
	pos := map[string]int{}
	for i, id := range store.createOrder {
		pos[id] = i
	}

	for _, id := range store.createOrder {
		item := store.items[id]
		parentID := item.Location.ParentContainerItemID
		if parentID == "" {
			continue
		}
		parentPos, inOrder := pos[parentID]
		if !inOrder {
			// parent is a pre-existing item, not created by this CopyItem call
			continue
		}
		if parentPos >= pos[id] {
			t.Fatalf("child %q (pos %d) created before parent %q (pos %d)",
				id, pos[id], parentID, parentPos)
		}
	}
}

func TestCopyItemCleansUpWhenPersistFails(t *testing.T) {
	store := newFakeStore()
	store.vaults["vault-1"] = domain.Vault{
		ID: "vault-1", CharacterName: "Lyra",
		StrengthScore: 20, EncumbranceMode: domain.EncumbranceModeStandard,
	}
	bag, pouch, coins := threeLevelFixture()
	store.items[bag.ID] = bag
	store.items[pouch.ID] = pouch
	store.items[coins.ID] = coins
	store.failCreateAfter = 1 // first create succeeds, second fails

	svc := NewService(store)
	_, err := svc.CopyItem(context.Background(), "bag", domain.ItemLocation{
		Kind: domain.LocationKindVaultRoot, OwnerVaultID: "vault-1",
	})
	if err == nil {
		t.Fatalf("expected error when persist fails mid-subtree")
	}

	items, _ := store.ListItems(context.Background())
	if len(items) != 3 { // only original bag+pouch+coins remain
		t.Fatalf("expected 3 items after cleanup, got %d", len(items))
	}
}
