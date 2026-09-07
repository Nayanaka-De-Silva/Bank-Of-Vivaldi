# Bank of Vivaldi

A self-hosted inventory manager for Dungeon Masters running Dungeons & Dragons 5th
Edition. Bank of Vivaldi tracks player-character vaults, nested containers, coin
purses, and a DM-only item compendium, and does the encumbrance and treasure-value
arithmetic so you don't have to.

It is written in Go with the standard library doing most of the work — no web
framework — backed by PostgreSQL, and served as plain server-rendered HTML with a
small JSON API alongside it.

> Part of a suite of self-hosted tabletop tools, each built in a different stack:
> **Bank of Vivaldi** (inventory · Go) ·
> [Many Faced God](https://github.com/Nayanaka-De-Silva/Many-Faced-God) (NPCs · Laravel) ·
> [Library of Netheril](https://github.com/Nayanaka-De-Silva/Library-Of-Netheril) (spells · TypeScript) ·
> [Manticore Arena](https://github.com/Nayanaka-De-Silva/Manticore-Arena) (combat tracker · TypeScript).

## Features

- **Character vaults** — one vault per player character or NPC. A vault records the
  owner's name, Strength score, and any carrying-capacity modifiers, then derives
  the character's carry limit and encumbrance thresholds automatically.
- **Nested containers** — bags, chests, and packs are themselves items that hold
  other items. Move an item between containers or vaults and every weight and value
  total upstream recomputes.
- **Coin purses** — copper, silver, electrum, gold, and platinum tracked per vault
  and rolled up to a single converted value. Coins are weightless by design, matching
  how most tables actually play.
- **DM compendium** — a weightless staging area for loot you intend to hand out.
  Transfer an item straight from the compendium into a character's vault when the
  moment comes.
- **Bulk entry** — paste a list of items, one per line, and preview the parse (with
  per-row errors) before committing. Supports quantity prefixes, denomination
  suffixes (`2gp`, `1,000gp`), and free-form attribute pairs.
- **Search, filter, and sort** across vaults and the compendium by name, category,
  and rarity.
- **Stack operations** — split, merge, copy, and move item stacks.
- **JSON API** — the same data over `/api/v1` for other tools to read (see below).

## Tech stack

| Layer | Choice |
|---|---|
| Language | Go 1.24 |
| HTTP | `net/http` standard library, no framework |
| Database | PostgreSQL 16 (`pgx/v5`) |
| Migrations | Plain SQL files, applied by `cmd/migrate` (or `AUTO_MIGRATE=true` on boot) |
| UI | Server-rendered `html/template`, progressively enhanced |
| Styling | Tailwind CSS v4, compiled to a committed `web/static/app.css` via the standalone CLI (keeps the toolchain Node-free) |
| Tests | Go standard `testing`, table-driven |
| CI/CD | Woodpecker |
| Packaging | Multi-stage Docker image |

## Architecture

The codebase follows a layered, dependency-inverted design:

```
cmd/                    entrypoints — server, migrate
internal/
  domain/               pure business rules — calculations, bulk parsing,
                        currency, filters, value types. No I/O.
  application/          use-case services orchestrating the domain
  config/               environment parsing
  infrastructure/
    postgres/           the store implementation + SQL migrations
    httpui/             server-rendered HTML handlers
    httpapi/            JSON API handlers, DTOs, error envelopes
web/
  templates/            html/template files
  tailwind/             Tailwind input; output is committed under static/
  static/               generated CSS, embedded into the binary at build time
```

`domain` knows nothing about HTTP or SQL. `application` depends on `domain` and on
storage interfaces. `infrastructure` supplies the concrete implementations. The
HTML and JSON transports are two adapters over the same service layer, not two
apps.

## Quick start

Requires Docker and Docker Compose.

```bash
docker compose up --build
```

Then open <http://localhost:8080>. The database schema is applied on first boot
(`AUTO_MIGRATE=true` in `compose.yaml`).

## Local development

```bash
# run the test suite in the dev container
docker compose run --rm app go test ./...

# or, with a local Go toolchain
go test ./...
go build ./cmd/...

# apply migrations manually against a running database
go run ./cmd/migrate

# regenerate the stylesheet after editing templates or web/tailwind/input.css
make css        # downloads the standalone tailwindcss binary into bin/ on first run
```

`web/static/app.css` is a **committed generated artifact** — regenerate and commit
it whenever templates change. The standalone Tailwind binary in `bin/` is gitignored.

## Configuration

All configuration is environment-driven. Copy `.env.example` to `.env` for local
overrides.

| Variable | Default | Description |
|---|---|---|
| `HTTP_ADDR` | `:8080` | Address the server listens on |
| `APP_PORT` | `8080` | Host port published by Compose |
| `DATABASE_URL` | `postgres://postgres:change-me@db:5432/bank_of_vivaldi?sslmode=disable` | PostgreSQL connection string |
| `POSTGRES_DB` / `POSTGRES_USER` / `POSTGRES_PASSWORD` | `bank_of_vivaldi` / `postgres` / `change-me` | Credentials for the bundled database container |
| `AUTO_MIGRATE` | `true` | Apply pending migrations on startup |

## JSON API

Bank of Vivaldi exposes a read-and-write JSON API under `/api/v1`, sharing the
service layer and database with the HTML UI.

**v1 has no authentication.** The trust boundary is the network: the container must
not be published on a port reachable from outside the trusted host. When another
container needs to call it, put both on a shared Docker network declared
`external: true` in the compose file.

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/api/v1/health` | Liveness check |
| `GET` | `/api/v1/vaults` | List vaults with computed weight/value/encumbrance. Filters: `?kind=pc\|npc`, `?q=name-fragment` |
| `POST` | `/api/v1/vaults` | Create a vault — `kind` (`"pc"` or `"npc"`) is required |
| `GET` | `/api/v1/vaults/{id}` | Vault detail: summary, item tree, links |
| `GET` | `/api/v1/vaults/{id}/items` | Browse a vault's items |
| `POST` | `/api/v1/vaults/{id}/items` | Add an item, inline or transferred from the compendium |
| `GET` | `/api/v1/vaults/{id}/links` | List external references registered against a vault |
| `POST` | `/api/v1/vaults/{id}/link` | Register / update an external app's reference (upsert) |
| `DELETE` | `/api/v1/vaults/{id}/link` | Remove one link — never deletes the vault |
| `GET` | `/api/v1/compendium/items` | Browse / filter the compendium |

Vault deletion is deliberately **not** in the API — it stays a considered action in
the UI.

**Envelopes.** List endpoints return
`{"data": [...], "meta": {"page", "pageSize", "totalItems", "totalPages"}}`;
single resources return the raw object; errors return
`{"error": {"code", "message", "details"?}}` with a matching HTTP status.

**Units.** Weight is stored in hundredths of a pound; value in copper pieces — the
domain's internal units, exposed as-is.

### Bulk item format

One item per line:

```text
2x Rope | equipment | mundane | 10 | 100
Lantern | equipment | mundane | 1 | 500
Gemstone | treasure | rare | 0.1 | 5000
```

Fields, in order: name (with optional quantity prefix), category, rarity, weight in
pounds, value in copper pieces, optional description, optional `key=value`
attributes.

## How it fits the suite

The `link` endpoints let another tool register a back-reference against a vault —
for example, [Many Faced God](https://github.com/Nayanaka-De-Silva/Many-Faced-God)
attaching an NPC's loadout to the NPC record it manages. Links are upserted and
removed independently of the vault itself.

## Deployment

`Dockerfile` produces a multi-stage image. `.woodpecker.yml` runs
`go test ./...` and `go build ./cmd/...` on every push and pull request, then builds
the runtime image and redeploys via `docker-compose.prod.yml` on pushes to
`master` — talking directly to the runner host's Docker socket, no registry.

The deploy step manages a `/opt/stacks/bank-of-vivaldi` directory on the runner
host; copy `.env.example` there as `.env` with production values before enabling
deploys.

## Content and licensing

The application code and schema are original and are released under the
[MIT License](LICENSE). No text, stat blocks, or item data from Wizards of the
Coast products are bundled with this repository — the model is a generic inventory
system, and any game content is supplied by the operator from sources they are
licensed to use.
