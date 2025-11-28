<!-- Copilot instructions for the go-poc repository -->

# Copilot instructions — go-poc

Purpose: quickly orient an AI coding agent to be productive in this Go microservice.

- Repo summary: small Go REST backend (GORM + SQLite) for restaurant Tables, Cards and Items, plus a Reservation flow.
- Key packages: `cmd/server` (entry), `internal/server` (routes), `internal/handlers` (HTTP handlers), `internal/db` (GORM wrapper), `internal/models` (domain types), `scripts/` (migration helpers), `docs/` (diagrams).

Quick run / test commands
- Start server locally: `go run ./cmd/server` (binds to port 8080 by default via `server.Run(8080)`).
- Tests: `go test ./...` (tests use in-memory sqlite: `file::memory:?cache=shared`).
- Migration script: `go run ./scripts/migrate_reservations_to_join.go --dry-run` (see flags `--db`, `--dry-run`, `--backup`).

Architecture & important patterns
- HTTP router: `chi` router configured in `internal/server/server.go`. Routes are declared there and wired to `handlers.Handler`.
- DB: GORM with `sqlite` driver. The DB wrapper is `internal/db.DB` which embeds `*gorm.DB` and runs `AutoMigrate` for models at startup.
- Models: see `internal/models/models.go`. Note:
  - `Reservation` uses a many-to-many relation to `Table` via `gorm:"many2many:reservation_tables;"`.
  - `Table.Status` is a string with expected values `free`, `reserved`, `in_use`.
- Handlers: `internal/handlers/handlers.go` implements all REST actions. Important practices used here:
  - Transactions: creation/checkin/checkout flows are wrapped in `tx := h.db.Begin()` and `Commit`/`Rollback` to avoid partial state.
  - Preloads: use `Preload("Items")` or `Preload("Tables")` before queries where associations are needed.
  - Overlap check for reservations uses a join against `reservation_tables` and rejects conflicting windows with `Status` checks.

Developer workflows & debugging notes
- Local DB file: `orders.db` in repo root. Handlers and server use `./orders.db` by default. Backups created in the repo (e.g., `orders.db.<timestamp>.bak`) during migration runs.
- When running tests, the in-memory sqlite DSN `file::memory:?cache=shared` is used — tests are closer to integration tests (they exercise GORM migrations and handlers).
- If you change models, keep `internal/db.New` AutoMigrate in mind — migrations occur at DB open in dev.

Conventions & repo-specific patterns
- HTTP status codes: handlers return explicit HTTP errors via `http.Error` and JSON-encode resources on success; follow existing patterns.
- Reservation statuses: `pending`, `confirmed`, `started`, `completed`, `cancelled` — handlers depend on these values for logic (overlap checks, transitions).
- Table status transitions: handlers set `status` to `reserved` during reservation create, `in_use` at check-in, and `free` at check-out. Keep this consistent for any new flows.
- Tests: add integration-style tests under `internal/handlers` using `httptest.NewServer` and an in-memory DB; follow existing tests for pattern.

Integration points & external dependencies
- Uses `gorm.io/gorm` + `gorm.io/driver/sqlite`.
- Uses `github.com/go-chi/chi/v5` router.
- No external services (auth, queues) are wired; keep feature additions isolated behind handlers.

Where to look first (recommended entry points)
- `internal/server/server.go` — routes and wiring.
- `internal/handlers/handlers.go` — core business logic and transactions.
- `internal/models/models.go` — schema and GORM tags.
- `internal/db/db.go` — DB initialization and AutoMigrate.
- `scripts/migrate_reservations_to_join.go` — example of utilities that operate directly on DB, includes `--dry-run` and `--backup` flags.

Notes for code changes
- Keep API routes stable: server.go lists the public endpoints; update both handlers and docs when changing.
- For any model change, add migrations or update `scripts/` helpers; tests will exercise AutoMigrate when using the in-memory DB.
- Preserve transaction semantics in handlers when updating reservation flows.

If unclear or you need more detail: point me to a specific file or flow and I will expand these instructions or generate starter tests/PRs.

Request feedback: are there missing details you want included (e.g., CI steps, linting, or release process)?
