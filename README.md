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

## Repo bootstrap and handoff

- This project is being bootstrapped as a **standalone Git repository** rooted at `bank-of-vivaldi`.
- The initial tracked scaffold already includes `cmd/server`, `cmd/migrate`, domain/application layers under `internal/`, PostgreSQL-backed infrastructure, HTML templates, Docker/Compose setup, and a Woodpecker pipeline.
- The local `D&D 5E - Player's Handbook.pdf` file is intentionally ignored and must stay out of version control and remote pushes.
- If native Go tooling is unavailable in the current shell, use the Compose commands above once Docker is stable again to validate the app from inside the container environment.

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
