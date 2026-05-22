CREATE TABLE IF NOT EXISTS app_settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

INSERT INTO app_settings (key, value)
VALUES ('default_encumbrance_mode', 'standard')
ON CONFLICT (key) DO NOTHING;

CREATE TABLE IF NOT EXISTS vaults (
    id UUID PRIMARY KEY,
    character_name TEXT NOT NULL,
    strength_score INTEGER NOT NULL,
    carry_modifier_lb INTEGER NOT NULL DEFAULT 0,
    encumbrance_mode TEXT NOT NULL,
    notes TEXT NOT NULL DEFAULT '',
    archived BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS purses (
    vault_id UUID PRIMARY KEY REFERENCES vaults(id) ON DELETE CASCADE,
    cp INTEGER NOT NULL DEFAULT 0,
    sp INTEGER NOT NULL DEFAULT 0,
    ep INTEGER NOT NULL DEFAULT 0,
    gp INTEGER NOT NULL DEFAULT 0,
    pp INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS items (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    slug TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    category TEXT NOT NULL,
    subcategory TEXT NOT NULL DEFAULT '',
    rarity TEXT NOT NULL,
    weight_hundredths_lb INTEGER NOT NULL DEFAULT 0,
    base_value_cp INTEGER NOT NULL DEFAULT 0,
    quantity INTEGER NOT NULL DEFAULT 1,
    is_container BOOLEAN NOT NULL DEFAULT FALSE,
    is_stackable BOOLEAN NOT NULL DEFAULT FALSE,
    is_equipped BOOLEAN NOT NULL DEFAULT FALSE,
    is_magical BOOLEAN NOT NULL DEFAULT FALSE,
    requires_attunement BOOLEAN NOT NULL DEFAULT FALSE,
    source_kind TEXT NOT NULL DEFAULT 'manual',
    details JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS item_locations (
    item_id UUID PRIMARY KEY REFERENCES items(id) ON DELETE CASCADE,
    location_kind TEXT NOT NULL,
    owner_vault_id UUID REFERENCES vaults(id) ON DELETE CASCADE,
    parent_container_item_id UUID REFERENCES items(id)
);

CREATE INDEX IF NOT EXISTS idx_items_name ON items (LOWER(name));
CREATE INDEX IF NOT EXISTS idx_items_category ON items (category);
CREATE INDEX IF NOT EXISTS idx_items_rarity ON items (rarity);
CREATE INDEX IF NOT EXISTS idx_item_locations_owner_vault ON item_locations (owner_vault_id);
CREATE INDEX IF NOT EXISTS idx_item_locations_parent_container ON item_locations (parent_container_item_id);
