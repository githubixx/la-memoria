# Foundation Red-Green Review

**Date**: 2026-08-31

## Approved Red Evidence

- `go test ./tests/testkit` passes, confirming the deterministic fakes compile.
- `go test ./bookmarker/usecase` fails because `bookmarker/model` and production use cases do not exist.
- `go test -tags=contract ./tests/contract` fails because `adapters/configyaml` and the delivery implementations do not exist.
- `go test -tags=integration ./tests/integration` fails because `adapters/postgres` and migrations do not exist.
- The isolated migration test starts PostgreSQL 18.6 through Testcontainers and fails reading the absent `0001_bookmarks.up.sql` migration.

The failures are caused by the intended missing Phase 2 behavior, not by an unavailable Go, Docker, PostgreSQL, or Node toolchain. The tests are approved for the foundational Green phase.

## Green Evidence (2026-09-01)

- `go build ./...` succeeds for the full module, including `cmd/bookmarker-web`
  and `cmd/bookmarker-cli`.
- `go test ./bookmarker/...` passes (authentication rotation/revocation,
  browser-session expiry, generic failure messages).
- `go test -tags=contract ./tests/contract/...` passes (strict YAML load
  rejection/resolution, HTTP security middleware, JSON Lines framing and
  stream authentication).
- `go test -tags=integration ./tests/integration/...` passes against real
  PostgreSQL 18.6 via Testcontainers: migration `0001` creates the declared
  schema, constraints, and indexes and rolls back cleanly; pair-specific
  login throttling is atomic and address-scoped; session revocation is
  immediately authoritative.
- `go test -race ./bookmarker/...` passes with no data races.
- `go vet ./...` and `gofmt -l .` report no issues.
- `npx playwright test --list` loads the configuration with zero specs,
  which is expected before Phase 3 (`tests/e2e/*.spec.ts` do not exist yet).

Foundational implementation is complete: `bookmarker/model`, `bookmarker/ports`,
authentication/password use cases, `adapters/configyaml`, `adapters/postgres`
(pool, migrations, auth/throttle store), `adapters/httpweb`, `adapters/jsonl`,
`adapters/observability`, the base template/CSS shell, non-production E2E
fixtures, and both `cmd/` composition roots are implemented and green.