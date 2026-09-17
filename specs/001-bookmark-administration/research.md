# Bookmarker Administration Research

**Feature**: `001-bookmark-administration`  
**Date**: 2026-08-31

## Go Runtime and Application Shape

**Decision**: Build Bookmarker with Go 1.27.0 as a standalone `bookmarker` library plus thin web-server and CLI executables. Use `net/http`, `html/template`, `context`, `log/slog`, and small explicit interfaces at adapter boundaries.

**Rationale**: Go 1.27.0 is the current stable release for this planning baseline. The standard library covers the HTTP, template, process, context, and structured-logging requirements while keeping the core portable. A library-first use-case layer satisfies the constitution and lets HTTP and CLI invoke identical behavior.

**Alternatives considered**: A full web framework was rejected because routing, middleware, and server-rendered responses do not require one. A web-only application was rejected because the constitution requires a reusable library and composable CLI.

## Architecture and Dependency Direction

**Decision**: Use Clean Architecture inside a Hexagonal boundary. Domain types and use cases depend only on ports for bookmarks, configuration, screenshots, sessions, login throttling, time, and identifiers. PostgreSQL, filesystem, YAML, `agent-browser`, HTTP, and JSON Lines CLI implementations are adapters wired only in `cmd/` composition roots.

**Rationale**: This isolates policy from delivery and infrastructure, supports deterministic unit tests, and prevents duplicated behavior between interfaces. Interfaces are introduced only where an external boundary exists; no generic Repository or UnitOfWork layer is added.

**Alternatives considered**: Active Record and handlers calling SQL directly were rejected because they bind feature behavior to one adapter. Generic repository abstractions were rejected because use-case-specific ports express the required operations with less indirection.

## PostgreSQL Persistence

**Decision**: Target PostgreSQL 18.6 and pgx v5.10.0 through `pgxpool`. Manage a versioned SQL schema with forward migrations and bound query parameters. Use PostgreSQL integration tests against a real disposable database.

**Rationale**: PostgreSQL 18.6 is the current stable production release; PostgreSQL 19 is still pre-release. pgx provides native PostgreSQL behavior, pooling, context cancellation, and efficient scanning without an ORM. SQL migrations make schema contracts reviewable and reproducible.

**Alternatives considered**: PostgreSQL 19 beta was rejected for production planning. An ORM was rejected because the model and queries are small and explicit SQL better exposes pagination, search, and transaction behavior. Mock-only persistence tests were rejected because they cannot verify SQL or schema compatibility.

## Bookmark and Tag Search

**Decision**: Normalize tags by trimming surrounding whitespace and applying Unicode-aware case folding for comparison, preserve one canonical display value, and enforce uniqueness only on normalized tag names and bookmark/tag pairs. Allow duplicate bookmark URLs. Implement description search with PostgreSQL `to_tsvector('simple', description) @@ plainto_tsquery('simple', input)` and combine tag and description predicates with AND. Order by `created_at DESC, id DESC`, cap the match set before paginating, and report when the cap is reached.

**Rationale**: The `simple` text-search configuration is language-neutral and `plainto_tsquery` gives all-word AND semantics while handling punctuation safely. Stable secondary ordering prevents page drift for equal timestamps. The schema preserves intentionally duplicated URLs while collapsing duplicate tags on one bookmark.

**Alternatives considered**: A unique URL constraint was rejected because the specification explicitly permits duplicate URLs. English stemming was rejected because it is not language-neutral. `ILIKE` chains were rejected because they scale poorly and mishandle tokenization. OR semantics were rejected because all entered description words must match.

## Configuration and Secret Handling

**Decision**: Load one strict `config.yaml` with unknown-field rejection, typed validation, and atomic replacement after a staged validation succeeds. Use `gopkg.in/yaml.v3` v3.0.1. Store the administrator username and an Argon2id salted password verifier in PHC string form in YAML, as explicitly required by the feature specification. Database passwords, cookie-signing material, and other runtime secrets are represented by environment-variable references in YAML and resolved from the environment; their values are never written back or displayed. The web Configuration view operates on a redacted DTO and cannot read or change credentials.

**Rationale**: Strict decoding catches misspellings instead of silently applying defaults. Storing the administrator's Argon2id PHC verifier in deployment-owned `config.yaml` follows the constitution's scoped allowance for configuration-based authentication because the feature specification explicitly requires it. The verifier remains sensitive: `config.yaml` must not be committed, must use restrictive filesystem permissions, and must never be exposed through application interfaces or operational records. Database credentials, session keys, tokens, and all other runtime secrets remain environment-provided. Atomic file replacement preserves the previous valid configuration if validation or persistence fails.

**Alternatives considered**: Storing plaintext credentials was rejected. Putting the password verifier only in an environment variable was rejected because it conflicts with FR-024 and the clarification. Exposing all YAML fields in the web form was rejected because FR-025 and FR-033 prohibit credential disclosure and editing. Permissive YAML decoding was rejected because invalid deployment settings could be silently ignored.

## Authentication, Sessions, Throttling, and CSRF

**Decision**: Verify Argon2id password hashes with constant-time comparison and issue a cryptographically random opaque session token after authentication. Store only a token digest and session state server-side; send the token in a `Secure`, `HttpOnly`, `SameSite=Lax` cookie with no `Expires` or `Max-Age`, and revoke it on sign-out. Persist failed-login events and pair-specific blocks by normalized username digest and trusted client address, using a database transaction to enforce 5 failures in 10 minutes and a 15-minute block. Protect unsafe HTTP methods with Go's same-origin protection plus a cryptographically random synchronizer token tied to the server-side session; login uses a short-lived anonymous form token and same-origin check.

**Rationale**: Opaque server-side sessions support immediate revocation and browser-session cookies satisfy FR-032. Database-backed throttling remains correct across processes and restarts. Layered same-origin and synchronizer-token checks cover cookie-authenticated requests without adopting a dependency with a known vulnerability. Source-address derivation trusts forwarding headers only from explicitly configured reverse proxies.

**Alternatives considered**: Persistent cookies and self-contained bearer tokens were rejected because browser-close expiry and immediate invalidation are required. In-memory throttling was rejected because it is inconsistent across restarts or replicas. Gorilla CSRF v1.7.3 was rejected due to GO-2025-3884. Relying only on `SameSite` was rejected because it is defense in depth, not the complete unsafe-request contract.

## Screenshot Capture and Filesystem Consistency

**Decision**: Implement screenshot capture as an adapter around external `agent-browser` v0.35.2. For each attempt, use `exec.CommandContext` with argument arrays, a unique `--session`, `--json`, a bounded timeout, bounded global concurrency, `open`, `wait --load networkidle`, `screenshot <staged-path> --full`, and guaranteed `close`. Permit public and private HTTP/HTTPS destinations, follow ordinary redirects, reject other schemes, and stage outputs under the configured screenshot root.

**Rationale**: An external browser adapter keeps browser automation outside the domain and follows the explicitly clarified network policy. Isolated sessions, context cancellation, bounded work, structured output, and guaranteed cleanup make failures observable and contain resource use.

**Alternatives considered**: Embedding a browser automation library in the core was rejected because it couples policy to infrastructure. Blocking private addresses was rejected because it contradicts the clarification. Shell command strings were rejected because argument arrays avoid quoting and injection errors.

## Cross-Resource Save and Delete Workflows

**Decision**: Use explicit staged filesystem operations with compensating cleanup rather than pretending PostgreSQL and the filesystem share one transaction. For create/update, capture to a unique staging file, commit the database reference, atomically rename the staged file into place, and compensate a failed promotion by clearing the reference and reporting capture failure. For replacement, retain the prior file until the new reference is committed and promoted, then remove the old file. For delete, atomically move the screenshot to a quarantine name, delete the database record in a transaction, restore the file if the transaction fails, and unlink the quarantined file after commit. Log and expose retryable cleanup failures without restoring a deleted bookmark.

**Rationale**: Rename-based staging on one filesystem provides reversible steps around the database commit and prevents partially created user-visible records. Explicit compensation documents the unavoidable failure windows and supports repair without adding a distributed transaction abstraction.

**Alternatives considered**: Deleting the database row and file independently was rejected because either order can produce visible inconsistency. Two-phase commit was rejected because ordinary filesystems do not participate and the complexity is unjustified. Storing screenshot binaries in PostgreSQL was rejected because the specification requires configured filesystem persistence.

## Server-Rendered UI and Progressive Enhancement

**Decision**: Render semantic HTML with `html/template`, modern CSS, and minimal vanilla JavaScript. Vendor htmx 2.0.10 locally for form and list updates. Every htmx URL is also a directly loadable full-page route; handlers return a fragment only when `HX-Request: true`, return the full document otherwise, and set `Vary: HX-Request`. Destructive actions remain POST/DELETE operations with server-side confirmation state and CSRF validation.

**Rationale**: Progressive enhancement keeps navigation, validation, authentication, and management usable without a client framework and satisfies the constitution's browser-technology constraint. Local vendoring avoids runtime CDN dependency and makes the deployed asset version reproducible.

**Alternatives considered**: A single-page framework was rejected because the workflows do not need client-side state management. CDN-hosted htmx was rejected because it weakens reproducibility and offline deployment. Fragment-only endpoints were rejected because direct navigation and non-JavaScript operation are required.

## CLI Contract

**Decision**: Expose Bookmarker use cases through a JSON Lines protocol on stdin/stdout. Accept exactly one versioned request object per line and emit exactly one success or structured error object per line in input order. Keep diagnostics and JSON `slog` records on stderr. Screenshot capture may complete asynchronously inside a request, but output ordering remains deterministic.

**Rationale**: JSON Lines is streamable, debuggable, and machine-composable while supporting typed operations and errors. Reusing the same application services as HTTP proves that behavior resides in the library.

**Alternatives considered**: Human-oriented subcommand output was rejected because it is harder to compose reliably. A separate CLI implementation was rejected because behavior would drift. Logging on stdout was rejected because it corrupts the protocol.

## Structured Logging and Sensitive Data

**Decision**: Emit JSON `slog` records by default with timestamp, level, operation, outcome, duration, request/correlation ID, and safe entity identifiers. Carry correlation IDs through HTTP, CLI, database, and capture calls. Redact passwords, password hashes, session and CSRF tokens, cookies, database credentials, environment values, form bodies, and sensitive full target URLs. Log capture destinations only as a scheme plus redacted host fingerprint unless explicit safe logging is configured.

**Rationale**: Consistent structured fields support diagnosis across adapters while the denylist prevents operational records from becoming a secret store. Correlation IDs connect frontend-triggered work to backend and external-process outcomes.

**Alternatives considered**: Free-form text logging was rejected because it is difficult to query. Full request and URL logging was rejected because bookmark destinations and credentials may be sensitive. Separate browser-console telemetry was rejected because minimal JavaScript does not require another logging system.

## Test Strategy and Browser Validation

**Decision**: Follow reviewed Red-Green-Refactor slices. Use Go unit tests for normalization, pagination, authentication policy, configuration validation, and use cases; contract and integration tests against real PostgreSQL and a controlled fake capture executable; and Playwright end-to-end tests for Chromium, Firefox, and a mobile Chromium viewport. Add focused `agent-browser` adapter smoke tests only where the executable is available. Verify both full-page and `HX-Request` forms of shared routes.

**Rationale**: Unit tests keep core feedback fast, real PostgreSQL tests verify SQL and transactional behavior, adapter tests verify process contracts, and Playwright covers the user-visible progressive-enhancement behavior. A controlled fake makes timeouts and malformed JSON deterministic without replacing smoke coverage of the real tool.

**Alternatives considered**: Mock-only database testing was rejected by the constitution and cannot validate PostgreSQL semantics. Chromium-only E2E was rejected because cross-browser behavior is required. Running the real internet and browser for every test was rejected because it is slow and nondeterministic.

## Versioning

**Decision**: Start the standalone library at semantic version `0.1.0`, independently version the CLI request envelope and HTTP contract as `v1`, and version the database schema through migrations beginning with `0001`. Before library `1.0.0`, incompatible public library changes require a MINOR version increment and migration notes; after `1.0.0`, they require a MAJOR version increment and migration guide. Incompatible CLI or HTTP contract changes require a new contract version regardless of the library version. Database changes always use forward migrations and compatibility notes.

**Rationale**: Version `0.y.z` explicitly identifies initial development without a stable public API while still making incompatible changes visible through MINOR increments and migration notes. Independently versioning the CLI and database schema prevents the library version from silently changing persisted or machine-consumed contracts.

**Alternatives considered**: Leaving interfaces unversioned was rejected because clients and persisted schemas would have no compatibility signal. Starting at `1.0.0` was rejected because the first feature remains pre-release.
