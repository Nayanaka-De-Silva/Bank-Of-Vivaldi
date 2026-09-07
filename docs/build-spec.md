# Bank of Vivaldi - Coding Agent Build Instructions

## 1. Purpose
Build **Bank of Vivaldi** as a web application for managing D&D 5e inventories for a Dungeon Master. The system must support:

- character vaults
- nested containers
- item catalogs
- a DM-only compendium
- purse and coin tracking
- weight and value calculations
- searching, sorting, and transferring items

This document replaces the loose concept note with an implementation-oriented specification suitable for a coding agent.

## 1.1 Current Handoff Status
- `bank-of-vivaldi` is intended to live as its own standalone Git repository rooted in this directory.
- The current implementation scaffold already contains:
  - `cmd/server` and `cmd/migrate`
  - domain logic and tests under `internal/domain`
  - application services and tests under `internal/application`
  - PostgreSQL persistence under `internal/infrastructure/postgres`
  - HTTP/UI wiring under `internal/infrastructure/httpui`
  - templates and static assets under `web/`
- Keep the local `D&D 5E - Player's Handbook.pdf` file untracked; it is not part of the repository payload.
- If an agent resumes work in an environment without native Go installed, prefer the documented Compose-based commands for build and test validation once container tooling is working again.

## 2. Product Goal
The application is a **single-operator inventory manager** for tabletop campaigns. The initial user is the DM, who manages:

- one or more player character vaults
- a shared item compendium
- item transfers between the compendium and characters
- character wealth and carried weight

The first release is **not** a public multi-tenant SaaS product. Treat it as a self-hosted DM tool.

## 3. Core Design Principles
- Use **Go** for the backend.
- Use a **relational database** because the domain is highly relational.
- Build the system with **test-driven development** where practical.
- Keep domain logic isolated from transport, storage, and UI concerns.
- Follow **SOLID** principles and prefer explicit interfaces over framework-heavy magic.
- Package the application for **Docker-based development and deployment**.
- Prepare the repo for **Woodpecker CI**.

## 4. Recommended Technical Shape
Unless the repository already dictates otherwise, implement the application with this baseline:

- **Backend:** Go HTTP service
- **Database:** PostgreSQL
- **Frontend:** server-rendered HTML with progressive enhancement, or a light SPA if the repo already uses one
- **API style:** JSON over HTTP
- **Migrations:** explicit SQL migrations
- **Containerization:** Docker and docker-compose for local development

Recommended backend layering:

1. `domain/` - entities, value objects, rules, services
2. `application/` - use cases and orchestration
3. `infrastructure/` - database, HTTP handlers, serialization
4. `web/` or `ui/` - templates or frontend assets

## 5. Scope for Version 1
Implement these features in the first version:

### 5.1 Vault Management
- Create, edit, archive, and view character vaults.
- Each vault belongs to one character.
- Each vault stores:
  - character name
  - strength score
  - optional carrying modifiers
  - active encumbrance mode
  - purse balances
- The user must be able to toggle the **Optional Encumbrance** rule from a topbar or navigation-pane settings control.
- The current encumbrance ruleset must be visible in the UI so the DM always knows whether standard carry rules or optional encumbrance rules are active.

### 5.2 Item Management
- Create, edit, view, search, sort, and delete items.
- Support generic items and specialized item metadata.
- Allow items to exist in:
  - a character vault root
  - a nested container
  - the compendium

### 5.3 Container Management
- Containers are items that can contain other items.
- Containers may be nested.
- A move must fail if the target container capacity would be exceeded.
- The UI must show total contained weight and total contained value.
- If moving an item between vaults or items will exceed the total container or vault limit, the application should display a warning to indicate that the limits have been met.

### 5.4 Compendium
- The compendium is a DM-owned storage area for prepared items.
- It has no strength-based carrying limit.
- Items can be transferred from the compendium into any character vault.

### 5.5 Purse and Currency
- Track copper, silver, electrum, gold, and platinum separately.
- Provide total value conversion in copper-equivalent and gold-equivalent views.
- Coins are tracked in the purse, not as normal inventory items.

### 5.6 Search and Sorting
- Search by item name, category, rarity, vault, and container.
- Sort by name, weight, value, category, rarity, and updated date.

### 5.7 Bulk Item Entry
- Provide a bulk item creation workflow so the DM can add many items in one operation.
- Bulk entry must support at least:
  - one item per line text input
  - optional quantity parsing
  - optional category, rarity, weight, and value fields
  - choosing the destination vault, compendium, or target container
- The workflow should favor speed over perfect structure and allow the DM to clean up item metadata afterward.
- Bulk entry must produce a preview before saving so the user can catch mistakes.

## 6. Explicit Non-Goals for Version 1
- No multiplayer player logins.
- No SRD or PHB item seeding bundled with the app.
- No copyrighted D&D text copied into shipped fixtures.
- No combat simulation, spell management, character sheets, or encounter tools.
- No marketplace or economy simulation beyond item and coin valuation.

## 7. PHB-Derived Domain Rules to Implement
Use the following rules as structured behavior, not as copied book text.

### 7.1 Currency
Support these denominations:

| Coin | Copper Value |
| --- | ---: |
| cp | 1 |
| sp | 10 |
| ep | 50 |
| gp | 100 |
| pp | 1000 |

Rules:
- 10 cp = 1 sp
- 5 sp = 1 ep
- 2 ep = 1 gp
- 10 gp = 1 pp
- 50 coins weigh 1 pound

Implementation note:
- Keep purse balances as integer counts.
- Provide deterministic conversion helpers.
- Purse totals should be queryable without mutating stored balances.

### 7.2 Carrying Capacity
Default carrying rules:

- carrying capacity = `strength * 15` pounds
- push / drag / lift = `strength * 30` pounds

Optional encumbrance mode:

- over `strength * 5`: encumbered
- over `strength * 10`: heavily encumbered
- over `strength * 15`: illegal carried state

Default behavior:
- Implement the **standard carrying rule** first.
- Support encumbrance as a vault-level toggle, but it may be disabled by default.
- Also provide an application-level UI control in the topbar or navigation pane so the DM can quickly enable or disable the optional encumbrance rules without digging into low-level configuration.
- Recommended behavior:
  - the topbar or navigation control sets the default ruleset for the application session or saved user preference
  - each vault stores its effective encumbrance mode so calculations remain explicit and reproducible
  - the UI must restate the active rule on each vault detail page

### 7.3 Containers
Use PHB container capacity guidance for common physical containers where appropriate:

| Container | Capacity |
| --- | --- |
| Backpack | 1 cubic foot / 30 lb |
| Chest | 12 cubic feet / 300 lb |
| Pouch | 0.2 cubic foot / 6 lb |
| Sack | 1 cubic foot / 30 lb |
| Bottle | 1.5 pints liquid |
| Flask or tankard | 1 pint liquid |
| Jug or pitcher | 1 gallon liquid |
| Waterskin | 4 pints liquid |
| Barrel | 40 gallons liquid / 4 cubic feet solid |
| Basket | 2 cubic feet / 40 lb |
| Bucket | 3 gallons liquid / 0.5 cubic foot solid |
| Iron pot | 1 gallon liquid |
| Vial | 4 ounces liquid |

Implementation rules:
- Capacity can be expressed by weight, volume, or liquid volume.
- For V1, **weight capacity is mandatory**.
- Volume support may be modeled now even if the UI only exposes weight initially.
- Containers must aggregate nested contents recursively.

### 7.4 Armor
Armor items should support:

- armor category: light, medium, heavy, shield
- base AC
- dexterity modifier behavior
- strength requirement if applicable
- stealth disadvantage flag
- item weight

Required base armor records supported by the schema:

- Light: padded, leather, studded leather
- Medium: hide, chain shirt, scale mail, breastplate, half plate
- Heavy: ring mail, chain mail, splint, plate
- Shield

Do not seed copyrighted descriptions. Store structure and numeric fields only.

### 7.5 Weapons
Weapon items should support:

- weapon class: simple melee, simple ranged, martial melee, martial ranged
- damage dice
- damage type
- properties
- normal range
- long range
- weight

Supported weapon properties:

- ammunition
- finesse
- heavy
- light
- loading
- reach
- special
- thrown
- two-handed
- versatile

### 7.6 Tools
Tool items should support:

- tool category
- proficiency-related notes
- cost
- weight

Minimum supported tool groups:

- artisan's tools
- disguise kit
- forgery kit
- gaming sets
- herbalism kit
- musical instruments
- navigator's tools
- poisoner's kit
- thieves' tools
- vehicles

### 7.7 Mounts and Vehicles
Support records for mounts and vehicles with:

- name
- type
- movement speed
- carrying capacity
- cost
- weight where meaningful

V1 requirement:
- store and display them as inventory-domain records
- do not build full mounted travel simulation

### 7.8 Trade Goods and Treasure
Support nonstandard valuables:

- trade goods
- gems
- jewelry
- art objects
- trinkets

Rules:
- they must support value without needing standard item mechanics like AC or damage
- they must be searchable and transferable like any other item

## 8. Item Model
Use a polymorphic or composition-friendly model. Do not cram every field into one flat struct without meaning.

### 8.1 Base Item Fields
Every item should support:

- `id`
- `name`
- `slug` or stable key
- `description`
- `category`
- `subcategory`
- `rarity`
- `weight_lb`
- `base_value_cp`
- `quantity`
- `is_container`
- `is_stackable`
- `is_equipped`
- `is_magical`
- `requires_attunement`
- `source_kind` (`manual`, `custom`, `imported`)
- timestamps

### 8.2 Specialized Metadata
Use dedicated tables or structured JSON for item-specific fields:

- `armor_details`
- `weapon_details`
- `container_details`
- `tool_details`
- `mount_details`
- `vehicle_details`
- `treasure_details`

The domain layer must hide storage details behind clear interfaces.

## 9. Ownership and Location Model
An item must have exactly one current location.

Supported location types:

1. compendium root
2. vault root
3. container item

Suggested location fields:

- `owner_vault_id` nullable
- `parent_container_item_id` nullable
- `in_compendium` boolean or location enum

Constraint:
- only one location target may be active at a time

## 10. Vault Model
Each vault should include:

- `id`
- `character_name`
- `strength_score`
- `carry_modifier_lb`
- `encumbrance_mode`
- `notes`
- timestamps

Encumbrance mode should support at least:

- `standard`
- `optional`

Derived fields or query projections:

- total carried weight
- total stored value
- carrying capacity
- push/drag/lift limit
- encumbrance state

## 11. Purse Model
The purse belongs to a vault, not to an item container.

Fields:

- `cp`
- `sp`
- `ep`
- `gp`
- `pp`

Derived helpers:

- total coin count
- total coin weight
- total purse value in copper
- total purse value in gold

Decide explicitly whether purse coin weight counts against carried weight. Default recommendation:

- **count purse coin weight toward vault carried weight**

This matches the PHB coin weight rule and keeps the inventory simulation coherent.

## 12. Rarity Model
Support these rarity levels:

- common
- uncommon
- rare
- very rare
- legendary
- artifact

Also support:

- mundane
- unknown

These extra values help for nonmagical equipment and incomplete records.

## 13. Business Rules
Implement these rules explicitly:

1. Moving an item into a container must check container capacity.
2. Moving an item into a vault must check vault carrying rules, except for the compendium.
3. Nested container weight must roll up recursively.
4. Deleting a container with children must be blocked or require explicit recursive confirmation.
5. Stackable items must merge only when their effective properties match.
6. Currency conversions must never lose value due to floating point rounding.
7. Value calculations must use integer math in copper units.
8. Search must work across root items and nested items.
9. Archived vaults must be read-only by default.
10. Bulk item creation must validate all rows before commit or clearly report which rows failed and why.
11. The optional encumbrance toggle must immediately affect derived encumbrance state and capacity messaging in the UI.

## 14. UI Requirements
Provide at least these screens:

1. Dashboard
2. Vault list
3. Vault detail
4. Item detail
5. Item create/edit form
6. Bulk item entry view
7. Container contents view
8. Compendium view
9. Transfer workflow
10. Search/results view

Global navigation or topbar requirements:

- include a visible settings or rules control
- allow toggling **Optional Encumbrance** on or off from that control
- show the currently active ruleset in a way that is obvious at a glance
- place the bulk item entry action in the topbar, navigation pane, compendium view, or vault view so it is easy for the DM to reach during data entry sessions

Vault detail should display:

- total carry weight
- max carry capacity
- encumbrance state
- active encumbrance ruleset
- purse summary
- root items
- nested containers

Bulk item entry UX requirements:

- support rapid keyboard-driven entry
- allow the DM to paste many lines at once
- show a parsed preview grid before commit
- allow destination selection before save
- surface validation errors per row instead of failing silently
- allow save as draft or cancel if the implementation already supports drafts; otherwise normal preview-and-commit is acceptable for V1

## 15. API / Handler Requirements
At minimum, implement endpoints or handlers for:

- vault CRUD
- item CRUD
- bulk item create
- compendium listing
- move item
- split stack
- merge stack
- adjust purse
- set encumbrance mode
- search inventory
- list container contents
- get computed vault summary

Prefer idempotent update semantics where practical.

## 16. Data Persistence Guidance
Suggested relational tables:

- `app_settings` or `user_preferences`
- `vaults`
- `purses`
- `items`
- `item_locations`
- `bulk_import_batches` if the implementation needs preview persistence
- `item_armor_details`
- `item_weapon_details`
- `item_container_details`
- `item_tool_details`
- `item_mount_details`
- `item_vehicle_details`
- `item_treasure_details`
- `tags` and `item_tags` if useful

Avoid baking everything into a single denormalized `items` table unless there is a clear performance reason.

## 17. Calculation Rules
Use deterministic domain services for:

- `ComputeVaultWeight`
- `ComputeVaultValue`
- `ComputeContainerWeight`
- `ComputeContainerValue`
- `ComputePurseValue`
- `ComputeCoinWeight`
- `ParseBulkItemInput`
- `PreviewBulkItemCreation`
- `CanMoveItem`
- `GetEncumbranceState`

These calculations must be unit tested directly without requiring HTTP or database setup.

## 18. Testing Requirements
Write tests for:

- currency conversion
- coin weight
- carrying capacity
- encumbrance thresholds
- switching between standard and optional encumbrance rules
- nested container aggregation
- container over-capacity rejection
- vault over-capacity rejection
- item transfer between compendium and vault
- item transfer between containers
- bulk item parsing
- bulk item preview validation
- bulk item commit behavior
- stack merge and split behavior
- search across nested storage
- topbar or navigation control for encumbrance settings if UI tests exist in the project

Testing layers:

1. domain unit tests
2. repository tests
3. HTTP handler or integration tests

## 19. Docker and Deployment Requirements
Provide:

- `Dockerfile`
- local compose setup
- environment variable configuration
- database migration step
- test command runnable in container

Separate application code from container and deployment configuration.

## 20. CI Requirements
Prepare a Woodpecker pipeline that runs:

1. dependency setup
2. tests
3. build
4. image build if the repo is configured for it

## 21. Copyright and Content Safety Requirements
Do **not** ship copyrighted PHB prose, tables, or seeded item text.

Allowed:

- original schema design
- original code
- original UI
- numeric or structural rules implemented as application logic
- user-entered records

Not allowed:

- bundling official item descriptions from Wizards of the Coast
- copying PHB text into source files, fixtures, or UI help text

## 22. Recommended Build Order
Implement in this order:

1. domain models and calculation services
2. database schema and migrations
3. repository layer
4. vault and purse CRUD
5. item CRUD
6. container movement logic
7. compendium workflow
8. bulk item entry workflow
9. search and filtering
10. UI polish
11. Docker and CI completion

## 23. Acceptance Criteria
The initial build is acceptable when all of the following are true:

1. A DM can create a character vault with a strength score.
2. A DM can create items and place them in a vault, a nested container, or the compendium.
3. The system computes carried weight and total value correctly.
4. The system computes coin value and coin weight correctly.
5. The user can toggle Optional Encumbrance from the topbar or navigation pane and the vault calculations update accordingly.
6. The system blocks illegal transfers that exceed capacity.
7. The system supports armor, weapons, tools, treasure, and generic gear without schema hacks.
8. The UI allows searching and sorting across stored items.
9. The DM can add many items in one bulk workflow with preview and validation before commit.
10. The project runs locally through Docker.
11. Tests cover the main domain rules.

## 24. Final Instruction to the Coding Agent
Build the application as a clean, production-shaped Go web project with strong domain modeling, explicit tests, and no bundled copyrighted D&D content. Prioritize correctness of inventory calculations, nested storage behavior, and a maintainable architecture over flashy UI.
