# Laguna Escondida Backend — Agent Rules

Go backend, **Hexagonal Architecture (Ports & Adapters)**. This file is the always-loaded rule set: keep it short. For the full blueprint (stack, aggregates, event bus, DI wiring, conventions) read **`docs/playbooks/ARCHITECTURE.md`** on demand — don't duplicate it here.

Stack: Gin (HTTP) · GORM + Postgres · Watermill (in-memory events) · shopspring `decimal` for money · mockery for port mocks · JWT + admin API key.

## Golden rules (non-negotiable)

- **Domain never imports platform.** Dependencies flow inward: `platform → domain`, never the reverse.
- **No `interface{}` / `any`** in signatures, ports, or interfaces. Use concrete types — enforce safety at compile time. Only if the user explicitly asks.
- **Never hand-edit mocks** in `internal/domain/ports/mocks/`. Run `make generate-mocks` (or `make regenerate-mocks` if it fails) after changing any port.
- **Ask before modifying existing tests** when changing a service's behavior. Bug fixes: update tests to cover the fix.
- **Never test aggregates directly** — exercise them through service tests. No test files under `domain/aggregate/`.
- **Run `make lint` after every change** (and `go build ./...`, `go test ./...`). Fix findings before calling work done.

## Layout & the dependency rule

```
internal/
├── domain/        # pure business logic — no platform imports
│   ├── ports/     # interfaces (contracts)
│   ├── service/   # use cases; depend on ports, never concrete adapters
│   ├── aggregate/ # DDD aggregates + value objects (see ARCHITECTURE.md)
│   ├── dto/       # plain structs, no logic
│   └── error/     # domain errors
└── platform/      # adapters — may import domain
    ├── handler/   # Gin HTTP handlers (thin: bind → call service → map errors)
    ├── postgres/  # GORM repositories (implement ports) + migrations
    └── dto/       # adapter-only structs
```

Import matrix:

- ✅ `platform/*` → `domain/*` (handlers use `domain/service`; repositories implement `domain/ports`)
- ✅ `platform/dto` → `domain/dto` (and stdlib only)
- ❌ `domain/*` → `platform/*` (any direction into platform)
- ❌ `domain/dto` → anything outside its own package except stdlib
- ❌ `platform/dto` → any `platform/*` except stdlib

## Layer responsibilities (terse)

- **ports/** — interface definitions only, focused and single-purpose. Accept/return domain types (aggregates, DTOs), never GORM models.
- **service/** — orchestration + business rules; receive ports via constructor injection (`NewXxxService(...)`). Return DTOs to handlers.
- **handler/** — validate/bind request, call a service (never a repository), map domain errors to HTTP status. Keep thin.
- **postgres/repository/** — implement a port; own all SQL/GORM, model↔DTO mapping; constructor returns the port type, not the concrete struct.

## Conventions

- `context.Context` is the first param of every repository/service/handler method.
- Errors: domain errors from `domain/error/` for business failures; wrap with `fmt.Errorf("...: %w", err)`. Don't log in the domain layer — log in adapters.
- Money is `decimal.Decimal`, never `float64`. IDs are UUID v7 strings. Soft-delete via `deleted_at`.
- Naming: `XxxService`, `XxxHandler`, `XxxRepository`; interfaces have no `Interface` suffix.

## Comments

Default to **none** — let naming and structure carry it. Add a comment only when the *why* is non-obvious (a business rule, a trade-off, a workaround). Never narrate *what/how* the code does. 1–2 lines max.

```go
// Opaque 401 so the response can't enumerate usernames; real reason logged server-side.
```

## Testing

- Every service has `*_service_test.go` alongside it; target 90%+ on service methods.
- Use testify (`assert`, `require`, `mock`, `suite`); mock all port dependencies; table-driven for similar scenarios.
- Cover success, error, edge, and calculation cases.
- Name: `Test{Service}_{Method}_{Scenario}`.

## Adding a feature

1. DTOs → `domain/dto/`  2. Port → `domain/ports/`  3. Use case → `domain/service/` **+ tests (mandatory)**
4. `make generate-mocks`  5. Repository → `platform/postgres/repository/`  6. Handler → `platform/handler/`
7. Wire in `cmd/main.go`  8. If it's an endpoint: update API docs (below).

## API documentation

Endpoints are documented for the frontend in `docs/api/{entity}.md`. **Create/update it whenever you add or change an endpoint, path, request/response schema, validation, or error.** Follow the format of **`docs/api/products.md`** (endpoint table → per-endpoint request/response/errors with realistic JSON), and add a working example to `docs/examples/curl-examples.md`.

## Make targets

`make lint` · `lint-fix` · `test` · `test-ci` · `test-acceptance` · `generate-mocks` · `regenerate-mocks` · `run` · `migrate-up` / `migrate-down` / `new-migration`.

Remember: **domain is the heart, platform is the shell** — keep domain pure and independent.
