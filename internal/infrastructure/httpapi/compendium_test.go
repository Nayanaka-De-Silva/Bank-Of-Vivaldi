package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"bank-of-vivaldi/internal/domain"
)

func TestBrowseCompendiumEnvelope(t *testing.T) {
	server, store := newTestServer()
	store.items["rope"] = domain.Item{
		ID: "rope", Name: "Rope", Slug: "rope", Category: "equipment",
		Rarity: domain.RarityMundane, Quantity: 1, WeightHundredthsLB: 1000, BaseValueCP: 100,
		SourceKind: domain.SourceKindManual,
		Location:   domain.ItemLocation{Kind: domain.LocationKindCompendiumRoot},
		CreatedAt:  time.Now(), UpdatedAt: time.Now(),
	}

	rec := doJSON(t, server.Routes(), http.MethodGet, "/compendium/items", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}

	var envelope struct {
		Data []itemEntryResponse `json:"data"`
		Meta listMeta            `json:"meta"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if len(envelope.Data) != 1 || envelope.Data[0].Item.Name != "Rope" {
		t.Fatalf("unexpected entries: %+v", envelope.Data)
	}
}

func TestBrowseCompendiumFilterByCategory(t *testing.T) {
	server, store := newTestServer()
	store.items["rope"] = domain.Item{
		ID: "rope", Name: "Rope", Slug: "rope", Category: "equipment",
		Rarity: domain.RarityMundane, Quantity: 1, SourceKind: domain.SourceKindManual,
		Location:  domain.ItemLocation{Kind: domain.LocationKindCompendiumRoot},
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	store.items["dagger"] = domain.Item{
		ID: "dagger", Name: "Dagger", Slug: "dagger", Category: "weapon",
		Rarity: domain.RarityMundane, Quantity: 1, SourceKind: domain.SourceKindManual,
		Location:  domain.ItemLocation{Kind: domain.LocationKindCompendiumRoot},
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}

	rec := doJSON(t, server.Routes(), http.MethodGet, "/compendium/items?category=weapon", nil)
	var envelope struct {
		Data []itemEntryResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(envelope.Data) != 1 || envelope.Data[0].Item.Name != "Dagger" {
		t.Fatalf("expected only Dagger, got %+v", envelope.Data)
	}
}

func TestBrowseVaultScopesToOwnItems(t *testing.T) {
	server, store := newTestServer()
	store.vaults["v1"] = domain.Vault{ID: "v1", CharacterName: "Aria", StrengthScore: 10, EncumbranceMode: domain.EncumbranceModeStandard}
	store.items["rope"] = domain.Item{
		ID: "rope", Name: "Rope", Slug: "rope", Category: "equipment",
		Rarity: domain.RarityMundane, Quantity: 1, SourceKind: domain.SourceKindManual,
		Location:  domain.ItemLocation{Kind: domain.LocationKindVaultRoot, OwnerVaultID: "v1"},
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	store.items["dagger"] = domain.Item{
		ID: "dagger", Name: "Dagger", Slug: "dagger", Category: "weapon",
		Rarity: domain.RarityMundane, Quantity: 1, SourceKind: domain.SourceKindManual,
		Location:  domain.ItemLocation{Kind: domain.LocationKindCompendiumRoot},
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}

	rec := doJSON(t, server.Routes(), http.MethodGet, "/vaults/v1/items", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	var envelope struct {
		Data []itemEntryResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(envelope.Data) != 1 || envelope.Data[0].Item.Name != "Rope" {
		t.Fatalf("expected only the vault's own item, got %+v", envelope.Data)
	}
}

func TestBrowseVaultUnknownVaultIsNotFound(t *testing.T) {
	server, _ := newTestServer()
	rec := doJSON(t, server.Routes(), http.MethodGet, "/vaults/nope/items", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d (%s)", rec.Code, rec.Body.String())
	}
}
