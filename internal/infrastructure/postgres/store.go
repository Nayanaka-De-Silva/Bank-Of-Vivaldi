package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"bank-of-vivaldi/internal/domain"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type Store struct {
	db *sql.DB
}

func Open(ctx context.Context, dsn string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return db, nil
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) GetAppSettings(ctx context.Context) (domain.AppSettings, error) {
	var value string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM app_settings WHERE key = 'default_encumbrance_mode'`).Scan(&value)
	if err != nil {
		return domain.AppSettings{}, err
	}
	return domain.AppSettings{DefaultEncumbranceMode: domain.ParseEncumbranceMode(value)}, nil
}

func (s *Store) SetDefaultEncumbranceMode(ctx context.Context, mode domain.EncumbranceMode) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO app_settings (key, value)
		VALUES ('default_encumbrance_mode', $1)
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value
	`, string(mode))
	return err
}

func (s *Store) CreateVault(ctx context.Context, vault domain.Vault) (domain.Vault, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Vault{}, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO vaults (
			id, character_name, strength_score, carry_modifier_lb,
			encumbrance_mode, notes, archived, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
	`, vault.ID, vault.CharacterName, vault.StrengthScore, vault.CarryModifierLB, string(vault.EncumbranceMode), vault.Notes, vault.Archived, vault.CreatedAt, vault.UpdatedAt); err != nil {
		return domain.Vault{}, err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO purses (vault_id, cp, sp, ep, gp, pp)
		VALUES ($1, 0, 0, 0, 0, 0)
	`, vault.ID); err != nil {
		return domain.Vault{}, err
	}

	if err := tx.Commit(); err != nil {
		return domain.Vault{}, err
	}

	return s.GetVault(ctx, vault.ID)
}

func (s *Store) UpdateVault(ctx context.Context, vault domain.Vault) (domain.Vault, error) {
	_, err := s.db.ExecContext(ctx, `
		UPDATE vaults
		SET character_name = $2,
			strength_score = $3,
			carry_modifier_lb = $4,
			encumbrance_mode = $5,
			notes = $6,
			archived = $7,
			updated_at = $8
		WHERE id = $1
	`, vault.ID, vault.CharacterName, vault.StrengthScore, vault.CarryModifierLB, string(vault.EncumbranceMode), vault.Notes, vault.Archived, vault.UpdatedAt)
	if err != nil {
		return domain.Vault{}, err
	}
	return s.GetVault(ctx, vault.ID)
}

func (s *Store) GetVault(ctx context.Context, id string) (domain.Vault, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT v.id, v.character_name, v.strength_score, v.carry_modifier_lb,
		       v.encumbrance_mode, v.notes, v.archived, v.created_at, v.updated_at,
		       COALESCE(p.cp, 0), COALESCE(p.sp, 0), COALESCE(p.ep, 0), COALESCE(p.gp, 0), COALESCE(p.pp, 0)
		FROM vaults v
		LEFT JOIN purses p ON p.vault_id = v.id
		WHERE v.id = $1
	`, id)
	return scanVault(row)
}

func (s *Store) ListVaults(ctx context.Context, includeArchived bool) ([]domain.Vault, error) {
	query := `
		SELECT v.id, v.character_name, v.strength_score, v.carry_modifier_lb,
		       v.encumbrance_mode, v.notes, v.archived, v.created_at, v.updated_at,
		       COALESCE(p.cp, 0), COALESCE(p.sp, 0), COALESCE(p.ep, 0), COALESCE(p.gp, 0), COALESCE(p.pp, 0)
		FROM vaults v
		LEFT JOIN purses p ON p.vault_id = v.id
	`
	var rows *sql.Rows
	var err error
	if includeArchived {
		rows, err = s.db.QueryContext(ctx, query+` ORDER BY LOWER(v.character_name)`)
	} else {
		rows, err = s.db.QueryContext(ctx, query+` WHERE v.archived = FALSE ORDER BY LOWER(v.character_name)`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	vaults := []domain.Vault{}
	for rows.Next() {
		vault, err := scanVault(rows)
		if err != nil {
			return nil, err
		}
		vaults = append(vaults, vault)
	}

	return vaults, rows.Err()
}

func (s *Store) SavePurse(ctx context.Context, vaultID string, purse domain.Purse) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO purses (vault_id, cp, sp, ep, gp, pp)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (vault_id) DO UPDATE
		SET cp = EXCLUDED.cp,
		    sp = EXCLUDED.sp,
		    ep = EXCLUDED.ep,
		    gp = EXCLUDED.gp,
		    pp = EXCLUDED.pp
	`, vaultID, purse.CP, purse.SP, purse.EP, purse.GP, purse.PP)
	return err
}

func (s *Store) CreateItem(ctx context.Context, item domain.Item) (domain.Item, error) {
	if err := s.saveItem(ctx, item, true); err != nil {
		return domain.Item{}, err
	}
	return s.GetItem(ctx, item.ID)
}

func (s *Store) UpdateItem(ctx context.Context, item domain.Item) (domain.Item, error) {
	if err := s.saveItem(ctx, item, false); err != nil {
		return domain.Item{}, err
	}
	return s.GetItem(ctx, item.ID)
}

func (s *Store) GetItem(ctx context.Context, id string) (domain.Item, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT i.id, i.name, i.slug, i.description, i.category, i.subcategory,
		       i.rarity, i.weight_hundredths_lb, i.base_value_cp, i.quantity,
		       i.is_container, i.is_stackable, i.is_equipped, i.is_magical,
		       i.requires_attunement, i.source_kind, i.details, i.created_at, i.updated_at,
		       l.location_kind, l.owner_vault_id, l.parent_container_item_id
		FROM items i
		JOIN item_locations l ON l.item_id = i.id
		WHERE i.id = $1
	`, id)
	return scanItem(row)
}

func (s *Store) ListItems(ctx context.Context) ([]domain.Item, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT i.id, i.name, i.slug, i.description, i.category, i.subcategory,
		       i.rarity, i.weight_hundredths_lb, i.base_value_cp, i.quantity,
		       i.is_container, i.is_stackable, i.is_equipped, i.is_magical,
		       i.requires_attunement, i.source_kind, i.details, i.created_at, i.updated_at,
		       l.location_kind, l.owner_vault_id, l.parent_container_item_id
		FROM items i
		JOIN item_locations l ON l.item_id = i.id
		ORDER BY LOWER(i.name), i.created_at
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []domain.Item{}
	for rows.Next() {
		item, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, rows.Err()
}

func (s *Store) DeleteItem(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM items WHERE id = $1`, id)
	return err
}

func (s *Store) saveItem(ctx context.Context, item domain.Item, insert bool) error {
	details, err := item.Details.JSON()
	if err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if insert {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO items (
				id, name, slug, description, category, subcategory, rarity,
				weight_hundredths_lb, base_value_cp, quantity,
				is_container, is_stackable, is_equipped, is_magical,
				requires_attunement, source_kind, details, created_at, updated_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)
		`, item.ID, item.Name, item.Slug, item.Description, item.Category, item.Subcategory, string(item.Rarity), item.WeightHundredthsLB, item.BaseValueCP, item.Quantity, item.IsContainer, item.IsStackable, item.IsEquipped, item.IsMagical, item.RequiresAttunement, string(item.SourceKind), details, item.CreatedAt, item.UpdatedAt)
	} else {
		_, err = tx.ExecContext(ctx, `
			UPDATE items
			SET name = $2,
			    slug = $3,
			    description = $4,
			    category = $5,
			    subcategory = $6,
			    rarity = $7,
			    weight_hundredths_lb = $8,
			    base_value_cp = $9,
			    quantity = $10,
			    is_container = $11,
			    is_stackable = $12,
			    is_equipped = $13,
			    is_magical = $14,
			    requires_attunement = $15,
			    source_kind = $16,
			    details = $17,
			    updated_at = $18
			WHERE id = $1
		`, item.ID, item.Name, item.Slug, item.Description, item.Category, item.Subcategory, string(item.Rarity), item.WeightHundredthsLB, item.BaseValueCP, item.Quantity, item.IsContainer, item.IsStackable, item.IsEquipped, item.IsMagical, item.RequiresAttunement, string(item.SourceKind), details, item.UpdatedAt)
	}
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO item_locations (item_id, location_kind, owner_vault_id, parent_container_item_id)
		VALUES ($1, $2, NULLIF($3, '')::uuid, NULLIF($4, '')::uuid)
		ON CONFLICT (item_id) DO UPDATE
		SET location_kind = EXCLUDED.location_kind,
		    owner_vault_id = EXCLUDED.owner_vault_id,
		    parent_container_item_id = EXCLUDED.parent_container_item_id
	`, item.ID, string(item.Location.Kind), item.Location.OwnerVaultID, item.Location.ParentContainerItemID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func scanVault(scanner interface{ Scan(dest ...any) error }) (domain.Vault, error) {
	var vault domain.Vault
	var encumbrance string
	err := scanner.Scan(
		&vault.ID,
		&vault.CharacterName,
		&vault.StrengthScore,
		&vault.CarryModifierLB,
		&encumbrance,
		&vault.Notes,
		&vault.Archived,
		&vault.CreatedAt,
		&vault.UpdatedAt,
		&vault.Purse.CP,
		&vault.Purse.SP,
		&vault.Purse.EP,
		&vault.Purse.GP,
		&vault.Purse.PP,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Vault{}, fmt.Errorf("vault: %w", domain.ErrNotFound)
	}
	if err != nil {
		return domain.Vault{}, err
	}
	vault.EncumbranceMode = domain.ParseEncumbranceMode(encumbrance)
	return vault, nil
}

func scanItem(scanner interface{ Scan(dest ...any) error }) (domain.Item, error) {
	var item domain.Item
	var rarity string
	var source string
	var details []byte
	var locationKind string
	var ownerVaultID sql.NullString
	var parentContainerID sql.NullString

	err := scanner.Scan(
		&item.ID,
		&item.Name,
		&item.Slug,
		&item.Description,
		&item.Category,
		&item.Subcategory,
		&rarity,
		&item.WeightHundredthsLB,
		&item.BaseValueCP,
		&item.Quantity,
		&item.IsContainer,
		&item.IsStackable,
		&item.IsEquipped,
		&item.IsMagical,
		&item.RequiresAttunement,
		&source,
		&details,
		&item.CreatedAt,
		&item.UpdatedAt,
		&locationKind,
		&ownerVaultID,
		&parentContainerID,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Item{}, fmt.Errorf("item: %w", domain.ErrNotFound)
	}
	if err != nil {
		return domain.Item{}, err
	}

	item.Rarity = domain.ParseRarity(rarity)
	item.SourceKind = domain.ParseSourceKind(source)
	item.Location = domain.ItemLocation{
		Kind:                  domain.LocationKind(locationKind),
		OwnerVaultID:          ownerVaultID.String,
		ParentContainerItemID: parentContainerID.String,
	}
	if len(details) > 0 {
		if err := json.Unmarshal(details, &item.Details); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return domain.Item{}, err
		}
	}

	return item, nil
}
