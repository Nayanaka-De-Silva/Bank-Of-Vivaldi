-- Vault links let an external application register an interest in a vault
-- without owning it. Deleting a link never deletes the vault; deleting a vault
-- cascades its links away.
-- Every statement must stay idempotent: Migrate re-runs this file on each boot.
CREATE TABLE IF NOT EXISTS vault_links (
    id           UUID PRIMARY KEY,
    vault_id     UUID NOT NULL REFERENCES vaults(id) ON DELETE CASCADE,
    external_ref TEXT NOT NULL,
    label        TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_vault_links_vault_ref
    ON vault_links (vault_id, external_ref);

CREATE INDEX IF NOT EXISTS idx_vault_links_external_ref
    ON vault_links (external_ref);
