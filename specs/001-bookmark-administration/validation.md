# Validation Record

**Feature**: `001-bookmark-administration`  
**Recorded**: 2026-09-02

## Toolchain and Dependencies

| Command | Observed result |
| --- | --- |
| `go version` | Pass: Go `1.27.0-X:nodwarf5` on Linux amd64 |
| `docker version --format '{{.Server.Version}}'` | Pass: Docker server `29.7.2` |
| `agent-browser --version` | Pass: `0.35.2` |
| `node --version` / `npm --version` | Pass: Node `v24.20.0`, npm `12.0.2` |
| `go mod download && npm ci` | Pass: dependencies installed; npm audit found 0 vulnerabilities |

## T127 Quality Gate

| Command | Observed result |
| --- | --- |
| `npm run format && git diff --check` | Pass |
| `go test ./bookmarker/... ./adapters/... ./cmd/...` | Pass |
| `go test -tags=contract ./tests/contract/...` | Pass |
| `go test -tags=integration ./tests/integration/...` | Pass using disposable PostgreSQL containers |
| `go test -race ./...` | Pass |
| `go vet ./...` | Pass |

## T128 Browser Matrix

`npm run test:e2e` passed `78/78` scenarios in `57.9s` across Chromium,
Firefox, and mobile Chromium. The Playwright global lifecycle created an
isolated PostgreSQL 18.6 container, applied migration `0001`, started the
capture fixture at `127.0.0.1:18081`, configured
`BOOKMARKER_CAPTURE_BASE_URL`, and reseeded `bookmarks.sql` before each test.

Focused configuration, security, and accessibility coverage passed `36/36`
scenarios in `21.4s`. It includes direct navigation with JavaScript disabled,
an htmx fragment response with `Vary: HX-Request`, keyboard menu/focus checks,
mobile text containment, local CSS/htmx asset retrieval, authentication and
CSRF/origin rejection, capture preview and failure flows, pagination, and
configuration activation.

WebKit is not part of the supported matrix. It was removed after the host
could not launch it because required Linux runtime libraries were unavailable.

## T129 Quickstart Scenarios

The quickstart's executable setup, dependency, full quality-gate, and browser
commands passed as recorded above. Its workflow scenarios were exercised by
the following automated acceptance coverage:

| Scenario | Observed automated outcome |
| --- | --- |
| Public browse, tag/search, pagination, direct pages, and htmx | US1 scenarios pass across all supported projects |
| Login, session revocation, CSRF/origin protection, and unauthenticated writes | Security-boundary scenarios pass across all supported projects; contract/integration suite passes |
| Capture preview, capture failure/continuation, create, edit, cancellation, and confirmed deletion | US2 and US3 scenarios pass across all supported projects; real `agent-browser` smoke test passes |
| Configuration redaction, invalid candidates, durable valid candidate, default view, and favicon | US4 scenarios pass across all supported projects |
| PostgreSQL scale and query-plan target | Focused 100,000-bookmark test passes in `6.545s`; individual browse/search assertions enforce a 2-second bound and 10,000-result cap |
| CLI, migration, strict YAML, compensation, and logging contracts | Complete contract and integration suites pass |

Manual exploratory steps remain reproducible from `quickstart.md`; the
automated matrix supplies the recorded execution for their specified outcomes.
