# Implementation Plan: Bookmarker Administration

**Branch**: `001-bookmark-administration` | **Date**: 2026-08-31 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `specs/001-bookmark-administration/spec.md`

## Summary

Build Bookmarker as a reusable Go library that supports public bookmark browsing, tag filtering, language-neutral all-word search, and authenticated bookmark, screenshot, and deployment-configuration administration. Thin HTTP and JSON Lines CLI adapters invoke the same Clean Architecture use cases. PostgreSQL stores records and server-side session state, the configured filesystem stores screenshots, `agent-browser` performs bounded capture, and server-rendered templates enhanced with locally vendored htmx provide the web interface.

## Technical Context

**Language/Version**: Go 1.27.0

**Primary Dependencies**: Go standard library (`net/http`, `html/template`, `log/slog`, `context`, `os/exec`); pgx v5.10.0; `gopkg.in/yaml.v3` v3.0.1; locally vendored htmx 2.0.10; external `agent-browser` v0.35.2

**Storage**: PostgreSQL 18.6 for bookmarks, tags, sessions, and authentication throttle state; configured local filesystem for screenshots; one strict `config.yaml` for deployment settings

**Testing**: Go `testing` unit tests; real PostgreSQL contract/integration tests using a disposable Testcontainers-managed database; deterministic capture-process adapter tests; Playwright tests across Chromium, Firefox, and a mobile Chromium viewport

**Target Platform**: Linux server with PostgreSQL, a writable screenshot/configuration filesystem, and the `agent-browser` executable; modern desktop and mobile browsers

**Project Type**: Standalone Go library with web-service and streaming CLI delivery adapters

**Performance Goals**: Browse, tag-filter, and search pages render within 2 seconds for up to 100,000 bookmarks and configured caps up to 10,000; 95% of normally reachable screenshot targets yield a preview or explicit failure within 15 seconds

**Constraints**: Server-rendered HTML; progressive enhancement; minimal vanilla JavaScript; every htmx route supports direct full-page loading; fixed 10-item list pages; bounded capture time and concurrency; duplicate bookmark URLs allowed; public and private HTTP/HTTPS capture allowed; no password, hash, token, cookie, database secret, or sensitive full URL in logs

**Scale/Scope**: One deployment-managed administrator identity; four primary destinations; approximately 100,000 bookmarks; one PostgreSQL database; one screenshot root; one web process or a small horizontally scaled set sharing PostgreSQL and storage

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

### Pre-Design Gate

| Principle | Status | Plan Evidence |
| --- | --- | --- |
| I. Library-First Architecture | PASS | `bookmarker/model`, `bookmarker/ports`, and `bookmarker/usecase` form a standalone library. HTTP, CLI, PostgreSQL, YAML, filesystem, and capture logic remain adapters. |
| II. Composable Command-Line Interfaces | PASS | The library exposes a documented versioned JSON Lines stdin/stdout protocol with diagnostics only on stderr. |
| III. Test-First Delivery | PASS | Every implementation slice begins with a reviewed failing unit, contract, integration, or browser test and follows Red-Green-Refactor. |
| IV. Contract and Integration Verification | PASS | PostgreSQL, JSON Lines, HTTP/htmx, YAML, filesystem, and capture-process boundaries receive real or controlled contract tests; PostgreSQL tests do not use mocks. |
| V. Observable, Simple, and Compatible Systems | PASS | JSON `slog`, request correlation, explicit use-case ports, no generic Repository or UnitOfWork, semantic version `0.1.0`, CLI envelope `v1`, and schema migrations are planned. |
| Browser Technology Constraint | PASS | Semantic server-rendered HTML, modern CSS, locally vendored htmx, and minimal vanilla JavaScript avoid a frontend framework. |
| Secret Handling Constraint | PASS | The clarified feature specification explicitly requires the salted one-way administrator password verifier in `config.yaml`. |

**Gate result**: PASS. The only exception is required by FR-024 and the user's recorded clarification, is bounded to a salted Argon2id verifier, and is documented below. There are no unresolved constitution violations.

### Post-Design Re-Check

Phase 1 artifacts preserve the same boundaries: [data-model.md](data-model.md) keeps infrastructure details out of domain entities; [contracts/http.md](contracts/http.md) and [contracts/cli-jsonl.md](contracts/cli-jsonl.md) define both delivery adapters; and [quickstart.md](quickstart.md) requires reviewed failing tests plus real PostgreSQL and cross-browser validation. Screenshot staging and compensation are explicit use-case workflows rather than a generic distributed transaction abstraction. No additional exceptions or unjustified patterns were introduced.

**Post-design gate result**: PASS with the same approved, documented credential-verifier exception.

## Architecture

### Library Boundary

The importable `bookmarker` library owns domain models, validation, normalization, application errors, and these use cases:

- Browse and paginate bookmarks, optionally filtered by exact normalized tag.
- Search by tag and/or all description words with configured page and result caps.
- Authenticate, sign out, validate server-side sessions, and enforce pair-specific login throttling.
- Create, edit, and delete bookmarks while coordinating tags and screenshot compensation.
- Begin, retry, accept, discard, and inspect screenshot capture drafts.
- Read redacted configuration and validate, stage, activate, or reject allowed configuration changes.

Use cases depend on narrow ports: `BookmarkStore`, `SessionStore`, `LoginThrottleStore`, `ScreenshotStore`, `ScreenshotCapturer`, `ConfigurationStore`, `Clock`, and `IDGenerator`. Transactional bookmark/tag methods are expressed by the use-case-specific `BookmarkStore`; no generic repository or unit-of-work API is exposed.

### Adapters and Composition

- PostgreSQL adapter: SQL migrations and pgx implementations for bookmark, tag, session, and throttle ports.
- Filesystem adapter: staged, promoted, quarantined, restored, and removed screenshot files under a validated root.
- `agent-browser` adapter: isolated command sessions, JSON parsing, bounded context, concurrency, and guaranteed close.
- YAML adapter: strict parsing, environment-secret resolution, validation, redaction, and atomic replacement.
- HTTP adapter: routing, secure cookies, authentication and CSRF middleware, full-page/fragment negotiation, templates, forms, and static assets.
- JSON Lines adapter: versioned request decoding, use-case dispatch, result/error encoding, and stderr diagnostics.
- Composition roots: `cmd/bookmarker-web` and `cmd/bookmarker-cli` construct concrete adapters and inject them into use cases.

### Cross-Resource Workflows

Screenshot writes are staged before database references are committed and atomically promoted afterward. Replacement retains the old file until the new reference is durable. Deletion first quarantines the file, rolls it back if database deletion fails, and removes it after commit. Promotion or cleanup failures produce structured operational records and retryable repair state; they never expose a partially created bookmark or restore a successfully deleted bookmark. These workflows and failure windows receive integration tests.

## Interface Contracts

### HTTP and htmx

[contracts/http.md](contracts/http.md) defines public/protected routes, methods, form fields, query parameters, status codes, redirects, fragment negotiation, `Vary: HX-Request`, CSRF rules, authentication responses, and error preservation. Unsafe methods require same-origin validation and a synchronizer token. Every GET reached by htmx renders a complete page when loaded directly.

### CLI

[contracts/cli-jsonl.md](contracts/cli-jsonl.md) defines one `v1` JSON request and one result or error response per line, stable operation names, typed payloads, ordered output, and stderr-only diagnostics. Initial operations cover bookmark browse/search/create/update/delete, capture drafts, sign-in/session/sign-out, and redacted configuration read/validate/update.

## Test-First Strategy

1. For each use-case slice, write and review the smallest behavior-focused test and confirm it fails for the expected reason before implementation.
2. Add unit tests beside Go packages for URL and tag normalization, all-word query construction, pagination bounds, configuration validation/redaction, password verification, throttling windows, session lifecycle, and compensation decisions.
3. Add PostgreSQL integration tests for migrations, duplicate URL acceptance, normalized tag uniqueness, stable ordering, language-neutral AND search, cap-before-pagination behavior, atomic bookmark/tag writes, session revocation, and concurrent throttle updates.
4. Add contract tests for strict YAML handling, environment references, filesystem traversal rejection and compensation, `agent-browser` argument/JSON/timeout behavior, CLI JSON Lines ordering, HTTP full-page/fragment parity, CSRF rejection, and authorization of every write path.
5. Add Playwright scenarios for all four user stories in Chromium, Firefox, and a mobile Chromium viewport, including signed-out behavior, failure feedback, missing screenshots, pagination, direct route loads, and htmx enhancement.
6. Run focused tests after each green step, then the full Go, integration, CLI contract, and Playwright suites before feature approval.

## Logging and Operations

JSON `slog` is the default for both executables. Each operation records outcome, duration, correlation ID, and safe bookmark/capture identifiers. HTTP request IDs flow through use cases, pgx calls, and capture subprocesses; CLI request IDs are accepted or generated and returned. Passwords, hashes, session and CSRF tokens, cookies, database credentials, environment values, request bodies, and sensitive full URLs are always excluded. Capture logs use scheme plus a redacted host fingerprint. Configuration activation and screenshot compensation failures are explicit auditable events.

## Versioning Impact

This first implementation introduces library version `0.1.0`, CLI envelope `v1`, HTTP form/route contract `v1` for acceptance fixtures, and database schema migration `0001`. Before library `1.0.0`, incompatible public library changes require a MINOR version increment and migration notes. After `1.0.0`, incompatible public library changes require a MAJOR version increment and migration guide. Incompatible CLI or HTTP contract changes require a new contract version regardless of the library version. Database changes always use forward migrations and compatibility notes.

## Project Structure

### Documentation (this feature)

```text
specs/001-bookmark-administration/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── http.md
│   └── cli-jsonl.md
└── tasks.md                 # Created later by /speckit.tasks, not by this plan
```

### Source Code (repository root)

```text
bookmarker/
├── model/                   # Domain entities, value objects, validation, errors
├── ports/                   # Use-case-owned external boundary interfaces
└── usecase/                 # Application orchestration and authorization policy

adapters/
├── agentbrowser/            # External screenshot capture process
├── configyaml/              # Strict YAML and environment-reference configuration
├── filesystem/              # Screenshot staging and compensation
├── httpweb/                 # net/http delivery, middleware, view models
├── jsonl/                   # Streaming CLI protocol delivery
└── postgres/                # pgx stores and SQL migrations

cmd/
├── bookmarker-cli/
└── bookmarker-web/

web/
├── static/
│   ├── css/
│   ├── js/
│   └── vendor/htmx.min.js
└── templates/
    ├── layouts/
    ├── pages/
    └── fragments/

migrations/
├── 0001_bookmarks.up.sql
└── 0001_bookmarks.down.sql

tests/
├── contract/                # HTTP, CLI, YAML, filesystem, process contracts
├── integration/             # Real PostgreSQL and cross-resource workflows
├── e2e/                     # Playwright desktop/mobile scenarios
└── fixtures/                # Deterministic config, capture, and database data
```

**Structure Decision**: Use one Go module with a public library package tree, sibling infrastructure/delivery adapters, and two thin executables. Go unit tests stay beside their packages; cross-package contract, real-database integration, and browser tests live under `tests/`. This keeps dependency direction visible without introducing separate backend and frontend projects for a server-rendered application.

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
| --- | --- | --- |
| Salted one-way administrator password verifier stored in `config.yaml` instead of supplied through an environment variable | FR-024 and the 2026-08-31 credential clarification explicitly require the username and salted one-way hash in the single deployment configuration. The approved scope is one Argon2id PHC verifier; plaintext and all other secrets remain environment-provided, hidden, and unlogged. | An environment-only verifier complies with the general constitution rule but contradicts the authoritative feature requirement and the user's explicit clarification. Storing plaintext is prohibited and less secure. |
