package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"bank-of-vivaldi/internal/domain"
)

func TestAddVaultItemInlineCreatesItem(t *testing.T) {
	server, store := newTestServer()
	store.vaults["v1"] = domain.Vault{ID: "v1", CharacterName: "Aria", StrengthScore: 10, EncumbranceMode: domain.EncumbranceModeStandard}

	rec := doJSON(t, server.Routes(), http.MethodPost, "/vaults/v1/items", addVaultItemRequest{
		Name:               "Rope",
		Category:           "equipment",
		WeightHundredthsLB: 1000,
		BaseValueCP:        100,
		Quantity:           1,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (%s)", rec.Code, rec.Body.String())
	}

	var item itemResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &item); err != nil {
		t.Fatalf("decode item: %v", err)
	}
	if item.Location.OwnerVaultID != "v1" || item.Location.Kind != "vault" {
		t.Fatalf("unexpected item location: %+v", item.Location)
	}
}

func TestAddVaultItemInlineRejectsMissingName(t *testing.T) {
	server, store := newTestServer()
	store.vaults["v1"] = domain.Vault{ID: "v1", CharacterName: "Aria", StrengthScore: 10, EncumbranceMode: domain.EncumbranceModeStandard}

	rec := doJSON(t, server.Routes(), http.MethodPost, "/vaults/v1/items", addVaultItemRequest{Category: "equipment"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestAddVaultItemTransfersFromCompendium(t *testing.T) {
	server, store := newTestServer()
	store.vaults["v1"] = domain.Vault{ID: "v1", CharacterName: "Aria", StrengthScore: 10, EncumbranceMode: domain.EncumbranceModeStandard}
	store.items["rope"] = domain.Item{
		ID: "rope", Name: "Rope", Slug: "rope", Category: "equipment",
		Rarity: domain.RarityMundane, Quantity: 1, WeightHundredthsLB: 1000, BaseValueCP: 100,
		SourceKind: domain.SourceKindManual,
		Location:   domain.ItemLocation{Kind: domain.LocationKindCompendiumRoot},
		CreatedAt:  time.Now(), UpdatedAt: time.Now(),
	}

	rec := doJSON(t, server.Routes(), http.MethodPost, "/vaults/v1/items", addVaultItemRequest{SourceItemID: "rope"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (%s)", rec.Code, rec.Body.String())
	}

	var item itemResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &item); err != nil {
		t.Fatalf("decode item: %v", err)
	}
	if item.ID == "rope" {
		t.Fatalf("expected a copy with a new id, got the source item back")
	}
	if item.Location.OwnerVaultID != "v1" {
		t.Fatalf("unexpected location: %+v", item.Location)
	}

	// The source item must still exist in the compendium — copy, not move.
	if _, ok := store.items["rope"]; !ok {
		t.Fatalf("expected source item to survive a copy")
	}
}

func TestAddVaultItemMoveRelocatesSourceItem(t *testing.T) {
	server, store := newTestServer()
	store.vaults["v1"] = domain.Vault{ID: "v1", CharacterName: "Aria", StrengthScore: 10, EncumbranceMode: domain.EncumbranceModeStandard}
	store.items["rope"] = domain.Item{
		ID: "rope", Name: "Rope", Slug: "rope", Category: "equipment",
		Rarity: domain.RarityMundane, Quantity: 1, WeightHundredthsLB: 1000, BaseValueCP: 100,
		SourceKind: domain.SourceKindManual,
		Location:   domain.ItemLocation{Kind: domain.LocationKindCompendiumRoot},
		CreatedAt:  time.Now(), UpdatedAt: time.Now(),
	}

	rec := doJSON(t, server.Routes(), http.MethodPost, "/vaults/v1/items", addVaultItemRequest{
		SourceItemID: "rope", Mode: "move",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (%s)", rec.Code, rec.Body.String())
	}

	var item itemResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &item); err != nil {
		t.Fatalf("decode item: %v", err)
	}
	if item.ID != "rope" {
		t.Fatalf("expected the same item id after a move, got %q", item.ID)
	}
	if item.Location.OwnerVaultID != "v1" {
		t.Fatalf("unexpected location: %+v", item.Location)
	}
}

func TestAddVaultItemUnknownVaultIsNotFound(t *testing.T) {
	server, _ := newTestServer()
	rec := doJSON(t, server.Routes(), http.MethodPost, "/vaults/nope/items", addVaultItemRequest{
		Name: "Rope", Category: "equipment",
	})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d (%s)", rec.Code, rec.Body.String())
	}
}

// TestAddVaultItemRejectsContainerFromAnotherVault guards against placing an
// item into vault A by naming a containerId that actually belongs to vault B.
// application.resolveLocation would otherwise silently derive OwnerVaultID
// from the container's real parent, moving the item into B instead of A.
func TestAddVaultItemRejectsContainerFromAnotherVault(t *testing.T) {
	server, store := newTestServer()
	store.vaults["a"] = domain.Vault{ID: "a", CharacterName: "Aria", StrengthScore: 10, EncumbranceMode: domain.EncumbranceModeStandard}
	store.vaults["b"] = domain.Vault{ID: "b", CharacterName: "Bram", StrengthScore: 10, EncumbranceMode: domain.EncumbranceModeStandard}
	store.items["bag-b"] = domain.Item{
		ID: "bag-b", Name: "Bram's Bag", Slug: "brams-bag", Category: "container",
		Rarity: domain.RarityMundane, Quantity: 1, IsContainer: true, SourceKind: domain.SourceKindManual,
		Details:   domain.ItemDetails{Container: &domain.ContainerDetails{MaxWeightHundredthsLB: 100000}},
		Location:  domain.ItemLocation{Kind: domain.LocationKindVaultRoot, OwnerVaultID: "b"},
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}

	rec := doJSON(t, server.Routes(), http.MethodPost, "/vaults/a/items", addVaultItemRequest{
		Name: "Rope", Category: "equipment", ContainerID: "bag-b",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (%s)", rec.Code, rec.Body.String())
	}
	if _, ok := store.items["bag-b"]; !ok {
		t.Fatalf("bag-b should be untouched")
	}
	for _, item := range store.items {
		if item.Location.ParentContainerItemID == "bag-b" {
			t.Fatalf("no item should have been placed into vault B's container, found %+v", item)
		}
	}
}
