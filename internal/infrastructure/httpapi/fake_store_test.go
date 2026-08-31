package httpapi

import (
	"context"
	"fmt"

	"bank-of-vivaldi/internal/domain"
)

// fakeStore is a minimal application.Store implementation for httpapi handler
// tests. It mirrors internal/application/service_test.go's fakeStore, which
// this package cannot import directly (unexported, different package).
type fakeStore struct {
	settings domain.AppSettings
	vaults   map[string]domain.Vault
	items    map[string]domain.Item
	links    map[string][]domain.VaultLink
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		settings: domain.AppSettings{DefaultEncumbranceMode: domain.EncumbranceModeStandard},
		vaults:   map[string]domain.Vault{},
		items:    map[string]domain.Item{},
		links:    map[string][]domain.VaultLink{},
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
		return domain.Vault{}, fmt.Errorf("vault %q: %w", id, domain.ErrNotFound)
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
		return domain.Item{}, fmt.Errorf("item %q: %w", id, domain.ErrNotFound)
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

func (f *fakeStore) CreateVaultLink(_ context.Context, link domain.VaultLink) (domain.VaultLink, error) {
	for _, existing := range f.links[link.VaultID] {
		if existing.ExternalRef == link.ExternalRef {
			return existing, nil
		}
	}
	f.links[link.VaultID] = append(f.links[link.VaultID], link)
	return link, nil
}

func (f *fakeStore) DeleteVaultLink(_ context.Context, vaultID, externalRef string) error {
	links := f.links[vaultID]
	for i, existing := range links {
		if existing.ExternalRef == externalRef {
			f.links[vaultID] = append(links[:i:i], links[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("vault link %q: %w", externalRef, domain.ErrNotFound)
}

func (f *fakeStore) ListVaultLinks(_ context.Context, vaultID string) ([]domain.VaultLink, error) {
	links := make([]domain.VaultLink, len(f.links[vaultID]))
	copy(links, f.links[vaultID])
	return links, nil
}
