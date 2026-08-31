package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"bank-of-vivaldi/internal/domain"
)

// CreateVaultLink inserts a link, or returns the existing row when the same
// external reference is already registered against the vault. Relinking is an
// upsert rather than an error so callers can retry safely.
func (s *Store) CreateVaultLink(ctx context.Context, link domain.VaultLink) (domain.VaultLink, error) {
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO vault_links (id, vault_id, external_ref, label, created_at)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (vault_id, external_ref) DO NOTHING
	`, link.ID, link.VaultID, link.ExternalRef, link.Label, link.CreatedAt); err != nil {
		return domain.VaultLink{}, err
	}

	row := s.db.QueryRowContext(ctx, `
		SELECT id, vault_id, external_ref, label, created_at
		FROM vault_links
		WHERE vault_id = $1 AND external_ref = $2
	`, link.VaultID, link.ExternalRef)

	return scanVaultLink(row)
}

func (s *Store) DeleteVaultLink(ctx context.Context, vaultID, externalRef string) error {
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM vault_links
		WHERE vault_id = $1 AND external_ref = $2
	`, vaultID, externalRef)
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return fmt.Errorf("vault link %q: %w", externalRef, domain.ErrNotFound)
	}
	return nil
}

func (s *Store) ListVaultLinks(ctx context.Context, vaultID string) ([]domain.VaultLink, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, vault_id, external_ref, label, created_at
		FROM vault_links
		WHERE vault_id = $1
		ORDER BY created_at, external_ref
	`, vaultID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	links := []domain.VaultLink{}
	for rows.Next() {
		link, err := scanVaultLink(rows)
		if err != nil {
			return nil, err
		}
		links = append(links, link)
	}
	return links, rows.Err()
}

func scanVaultLink(scanner interface{ Scan(dest ...any) error }) (domain.VaultLink, error) {
	var link domain.VaultLink
	err := scanner.Scan(
		&link.ID,
		&link.VaultID,
		&link.ExternalRef,
		&link.Label,
		&link.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.VaultLink{}, fmt.Errorf("vault link: %w", domain.ErrNotFound)
	}
	if err != nil {
		return domain.VaultLink{}, err
	}
	return link, nil
}
