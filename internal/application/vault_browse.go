package application

import (
	"context"

	"bank-of-vivaldi/internal/domain"
)

// BrowseVault lists everything a vault owns -- its root items plus anything
// nested inside one of its containers -- narrowed by the same filters the
// compendium browser uses. CompendiumFilters is deliberately shared rather
// than duplicated: every field is location-agnostic (domain.CompendiumCriteria
// never inspects location), so a vault-specific wrapper type would add
// ceremony without adding meaning.
func (s *Service) BrowseVault(ctx context.Context, vaultID string, filters CompendiumFilters) (ItemBrowse, error) {
	if _, err := s.store.GetVault(ctx, vaultID); err != nil {
		return ItemBrowse{}, err
	}
	return s.browseItems(ctx, filters, vaultScopedItems(vaultID), "Vault root")
}

// vaultScopedItems returns every item a vault holds: root items plus anything
// nested inside one of its containers. This is a flat predicate, not a tree
// walk -- Service.resolveLocation stamps a container child's OwnerVaultID from
// its parent at save time, so a nested item's own OwnerVaultID already
// identifies the owning vault, exactly as compendiumScopedItems relies on a
// nested compendium item's OwnerVaultID staying empty.
func vaultScopedItems(vaultID string) func([]domain.Item) []domain.Item {
	return func(items []domain.Item) []domain.Item {
		scoped := make([]domain.Item, 0, len(items))
		for _, item := range items {
			if item.Location.OwnerVaultID != vaultID {
				continue
			}
			switch item.Location.Kind {
			case domain.LocationKindVaultRoot, domain.LocationKindContainer:
				scoped = append(scoped, item)
			}
		}
		return scoped
	}
}
