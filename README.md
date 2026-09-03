# Bank of Vivaldi

Bank of Vivaldi is a self-hosted Go web application for managing D&D-style inventory, vaults, containers, compendium items, and purse tracking for a Dungeon Master workflow.

## Stack

- Go backend
- PostgreSQL persistence
- Server-rendered HTML UI
- Docker and Compose for local development
- Woodpecker CI pipeline

## Quick start

```bash
docker compose up --build
```

Then open `http://localhost:8080`.

## Common commands

```bash
docker compose run --rm app go test ./...
docker compose run --rm app go build ./cmd/...
docker compose run --rm app go run ./cmd/migrate
```

## Woodpecker CI/CD

- CI runs `go test ./...` and `go build ./cmd/...` on pushes and pull requests.
- CD builds the runtime image on pushes to `master` and deploys with `docker-compose.prod.yml`.
- The deploy step manages `/home/krystler/containers/bank-of-vivaldi` on the Woodpecker runner host.
- Before enabling deploys, copy `.env.example` to `/home/krystler/containers/bank-of-vivaldi/.env` and set production values.

## Repo bootstrap and handoff

- This project is being bootstrapped as a **standalone Git repository** rooted at `bank-of-vivaldi`.
- The initial tracked scaffold already includes `cmd/server`, `cmd/migrate`, domain/application layers under `internal/`, PostgreSQL-backed infrastructure, HTML templates, Docker/Compose setup, and a Woodpecker pipeline.
- The local `D&D 5E - Player's Handbook.pdf` file is intentionally ignored and must stay out of version control and remote pushes.
- If native Go tooling is unavailable in the current shell, use the Compose commands above once Docker is stable again to validate the app from inside the container environment.

## API

Bank of Vivaldi exposes a JSON API under `/api/v1`, alongside the HTML UI. Both surfaces share the same service layer and database — the API is a second transport, not a separate app. See `internal/infrastructure/httpapi` for the implementation.

**v1 has no authentication.** Access control is the Docker network boundary only: the API container must not be published on a port reachable from outside the trusted host network. If another container needs to call it, declare that integration network as `external: true` in `docker-compose.prod.yml` — never wire it up with a manual `docker network connect`, which gets silently dropped on every `--force-recreate` deploy.

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/api/v1/health` | liveness check |
| `GET` | `/api/v1/vaults` | list vaults with computed weight/value/encumbrance; accepts `?kind=pc\|npc` and `?q=name-fragment` |
| `POST` | `/api/v1/vaults` | create a vault — **`kind` (`"pc"` or `"npc"`) is required** |
| `GET` | `/api/v1/vaults/{id}` | vault detail: summary (includes `kind`), item tree, links |
| `GET` | `/api/v1/vaults/{id}/items` | browse a vault's items (same filters as the compendium) |
| `POST` | `/api/v1/vaults/{id}/items` | add an item to a vault, inline or transferred from the compendium |
| `GET` | `/api/v1/vaults/{id}/links` | list external links registered against a vault |
| `POST` | `/api/v1/vaults/{id}/link` | register an external app's reference against a vault (upsert) |
| `DELETE` | `/api/v1/vaults/{id}/link` | remove one link — never deletes the vault itself |
| `GET` | `/api/v1/compendium/items` | browse/filter the compendium |

Vault deletion is intentionally **not** exposed over the API — it stays a deliberate action in the UI.

**Vault `kind` field:** Every vault has a `kind` — `"pc"` (Player Character) or `"npc"` (NPC). `POST /api/v1/vaults` returns a 400 with a `kind` field error when `kind` is omitted or unrecognised. Existing rows without a kind were backfilled to `"pc"` by migration `003_vault_kind.sql`. The `kind` field appears in every vault response object.

List endpoints return `{"data": [...], "meta": {"page", "pageSize", "totalItems", "totalPages"}}`; single-resource endpoints return the raw object; errors return `{"error": {"code", "message", "details"?}}` with a matching HTTP status. Weight is in hundredths of a pound and value is in copper pieces, matching the domain's internal units.

## Bulk item format

One item per line:

```text
2x Rope | equipment | mundane | 10 | 100
Lantern | equipment | mundane | 1 | 500
Gemstone | treasure | rare | 0.1 | 5000
```

Fields are:

1. name with optional quantity prefix
2. category
3. rarity
4. weight in pounds
5. value in copper pieces
