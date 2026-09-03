-- Adds a vault type column (pc or npc).
-- Idempotent: each statement is safe to re-run on every boot.
ALTER TABLE vaults ADD COLUMN IF NOT EXISTS kind TEXT;
UPDATE vaults SET kind = 'pc' WHERE kind IS NULL;
ALTER TABLE vaults ALTER COLUMN kind SET NOT NULL;
CREATE INDEX IF NOT EXISTS idx_vaults_kind ON vaults (kind);
