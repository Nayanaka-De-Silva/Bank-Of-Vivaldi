package application

import (
	"context"
	"fmt"

	"bank-of-vivaldi/internal/domain"

	"github.com/google/uuid"
)

type LinkVaultInput struct {
	VaultID     string
	ExternalRef string
	Label       string
}

// LinkVault registers an external application's reference against an existing
// vault. Relinking an already-registered reference returns the existing link
// rather than failing, so callers can treat it as an upsert.
func (s *Service) LinkVault(ctx context.Context, input LinkVaultInput) (domain.VaultLink, error) {
	link := domain.VaultLink{
		VaultID:     input.VaultID,
		ExternalRef: input.ExternalRef,
		Label:       input.Label,
	}.Normalize()

	if err := link.Validate(); err != nil {
		return domain.VaultLink{}, err
	}

	if _, err := s.store.GetVault(ctx, link.VaultID); err != nil {
		return domain.VaultLink{}, fmt.Errorf("vault %q: %w", link.VaultID, err)
	}

	link.ID = uuid.NewString()
	link.CreatedAt = s.now().UTC()

	return s.store.CreateVaultLink(ctx, link)
}

// UnlinkVault removes a single external reference from a vault. The vault and
// its contents are untouched — deleting a vault stays a deliberate UI action.
func (s *Service) UnlinkVault(ctx context.Context, vaultID, externalRef string) error {
	link := domain.VaultLink{VaultID: vaultID, ExternalRef: externalRef}.Normalize()
	if err := link.Validate(); err != nil {
		return err
	}

	if _, err := s.store.GetVault(ctx, link.VaultID); err != nil {
		return fmt.Errorf("vault %q: %w", link.VaultID, err)
	}

	return s.store.DeleteVaultLink(ctx, link.VaultID, link.ExternalRef)
}

func (s *Service) ListVaultLinks(ctx context.Context, vaultID string) ([]domain.VaultLink, error) {
	if _, err := s.store.GetVault(ctx, vaultID); err != nil {
		return nil, fmt.Errorf("vault %q: %w", vaultID, err)
	}
	return s.store.ListVaultLinks(ctx, vaultID)
}
