# Bookmarker Administration Quickstart

**Feature**: `001-bookmark-administration`  
**Purpose**: Runnable validation guide for the completed implementation  
**Contracts**: [HTTP and htmx](contracts/http.md), [CLI JSON Lines](contracts/cli-jsonl.md)  
**Model**: [data-model.md](data-model.md)

## Prerequisites

- Linux development host.
- Go 1.27.0.
- Docker-compatible container runtime for PostgreSQL integration tests.
- PostgreSQL 18.6 client tools for optional manual inspection.
- `agent-browser` 0.35.2 on `PATH` with its browser installed.
- Current Node.js LTS and npm for Playwright.
- Chromium and Firefox installed through Playwright.

Verify the toolchain:

```bash
go version
docker version
agent-browser --version
node --version
npm --version
```

Expected: each command succeeds; Go reports 1.27.0 and `agent-browser` reports 0.35.2. PostgreSQL integration tests start version 18.6 through Testcontainers rather than substituting an in-memory database.

## Install Dependencies

From the repository root after the implementation files exist:

```bash
go mod download
npm ci
npx playwright install chromium firefox
```

htmx is served from the checked-in `web/static/vendor/htmx.min.js`; the application must not need a CDN at runtime.

## Test-First Gate

Before each implementation slice, add and review its behavior-focused test, then run only that test and retain evidence that it fails for the expected missing behavior. Examples of focused commands are:

```bash
go test ./bookmarker/usecase -run TestCreateBookmark
go test -tags=integration ./tests/integration -run TestDuplicateBookmarkURLs
go test ./tests/contract -run TestHTTPBookmarkCreate
npx playwright test --project=chromium -g "administrator creates a bookmark"
```

Expected before implementation: the new test fails because the behavior is absent, not because the fixture or environment is broken. After the smallest implementation, rerun the same command to green before refactoring. Review must reject a feature slice with no recorded red step.

## Automated Validation

Run fast Go tests and static checks:

```bash
go test ./bookmarker/... ./adapters/... ./cmd/...
go vet ./...
```

Expected: normalization, validation, pagination, all-word query construction, authorization, session rotation/revocation, throttling windows, configuration redaction, and compensation decisions pass without network access.

Run real-boundary suites with Docker available:

```bash
go test -tags=contract ./tests/contract/...
go test -tags=integration ./tests/integration/...
```

Expected:

- PostgreSQL 18.6 migrations apply from an empty database and reach the declared schema version.
- Two bookmarks with the same URL persist as separate records.
- Tag normalization and bookmark/tag uniqueness hold under concurrent writes.
- Search uses language-neutral all-word AND semantics, stable newest-first ordering, cap-before-pagination, and exact normalized tags.
- Session revocation and transactional username/source-address throttling work across database connections.
- YAML rejects unknown/secret fields and preserves the last valid file after validation or replacement failure.
- Filesystem staging, promotion, quarantine, restore, and post-commit cleanup compensation survive injected failures.
- The capture adapter passes exact argument arrays, parses JSON, times out, limits concurrency, and always closes its unique session.
- HTTP full-page and `HX-Request: true` contracts return equivalent state with `Vary: HX-Request`; every unsafe path enforces authorization, origin, and CSRF checks.
- CLI stdout contains one response per request and no diagnostics; stderr contains safe correlated JSON logs.

## Local PostgreSQL and Configuration

Start an isolated manual database when browser validation is not using the Playwright-managed environment:

```bash
docker run --rm --detach \
  --name bookmarker-postgres \
  --publish 54329:5432 \
  --env POSTGRES_DB=bookmarker \
  --env POSTGRES_USER=bookmarker \
  --env POSTGRES_PASSWORD=bookmarker-test \
  postgres:18.6
```

Use the checked-in non-production fixture and environment references:

```bash
export BOOKMARKER_CONFIG="$PWD/tests/fixtures/e2e/config.yaml"
export BOOKMARKER_DB_PASSWORD='bookmarker-test'
export BOOKMARKER_ALLOW_MIGRATIONS='true'
```

The fixture documents its test-only administrator password next to its Argon2id verifier. It must use a temporary screenshot root and database password environment-variable **name**, never an inline database password. Production credentials must not be placed in the fixture.

Apply migrations through the deployment-authorized CLI mode:

```bash
printf '%s\n' \
  '{"version":"v1","id":"migrate-1","operation":"system.migrate","payload":{}}' \
  | go run ./cmd/bookmarker-cli
```

Expected stdout: one successful `v1` response containing starting and resulting schema versions. Expected stderr: safe JSON diagnostics. No SQL, password, or resolved environment value appears in either stream.

## Run the Web Application

```bash
go run ./cmd/bookmarker-web
```

Expected: the server listens on the fixture address, normally `http://127.0.0.1:8080`, emits startup JSON on stderr, and serves CSS and vendored htmx locally. Keep this process running while performing the manual and browser checks below.

## Public Browse and Search Scenario

1. Seed at least 25 bookmarks, including two records with the same URL but different descriptions/tags and mixed-case tag spellings.
2. Open `/bookmarks` without a session.
3. Follow page, Previous, Next, first, final, and ellipsis links.
4. Select a tag, then search with two description words in reverse order and mixed case.
5. Submit an empty search and request page 0 and a page beyond the end.

Expected:

- List is newest first with URL, description, creation date, then tags; no Edit/Delete controls appear.
- Each duplicate URL record remains independently visible.
- Exact tag filtering ignores surrounding whitespace/case, and description results contain every entered word regardless of order/case.
- Empty search returns field guidance, pages clamp to valid bounds, and configured maximum truncation is clearly identified.
- Direct GETs and htmx navigation show the same state without overlapping content or broken history.

## Authentication and Throttling Scenario

1. Load `/login` and inspect the session cookie attributes.
2. Submit four incorrect attempts for one username/source-address pair, then a fifth within 10 minutes.
3. Verify a different test source remains unblocked.
4. Attempt the blocked pair again, advance the controlled integration-test clock 15 minutes, and sign in correctly.
5. Sign out and replay the old cookie and CSRF token.

Expected: the cookie is `Secure`, `HttpOnly`, `SameSite=Lax`, and has no persistent expiry; the fifth failure creates only the pair-specific 15-minute block; messages do not distinguish username/password/block causes; the old session and CSRF token cannot perform a write after rotation or sign-out.

## Bookmark and Screenshot Scenario

1. Sign in and open `/bookmarks/new` directly.
2. Enter a local fixture-server URL such as `http://127.0.0.1:<fixture-port>/capture-target` to prove private HTTP targets are permitted.
3. Observe pending/ready state, remove one duplicate-normalized tag, and save.
4. Edit the URL and review a replacement capture before saving.
5. Repeat with the fixture capture endpoint made slow/unreachable; retry once, then explicitly continue without a screenshot.
6. Delete another bookmark, first cancelling and then confirming.

Expected: capture finishes or reports a safe failure within the 15-second acceptance budget; non-HTTP/HTTPS input never launches the process; drafts are bound to session/URL/bookmark; edit retains original creation time; cancellation has no effect; confirmed deletion removes browse/search state and uses repairable cleanup if injected file removal fails.

Exercise the external tool independently against the running app:

```bash
agent-browser --session bookmarker-smoke --json open http://127.0.0.1:8080/bookmarks
agent-browser --session bookmarker-smoke --json wait --load networkidle
agent-browser --session bookmarker-smoke --json screenshot /tmp/bookmarker-smoke.png --full
agent-browser --session bookmarker-smoke --json close
```

Expected: each command emits valid JSON, the screenshot is non-empty, and `close` releases the named session. Remove `/tmp/bookmarker-smoke.png` after inspection.

## Configuration Scenario

1. Open `/configuration` signed in and inspect page source plus htmx responses.
2. Change title, favicon, default view, screenshot root, non-secret database fields/environment reference name, page size, and maximum results.
3. Submit unsupported favicon content, an unwritable/traversing screenshot path, an unknown YAML field through the CLI, invalid search limits, and unreachable database settings.
4. Reload after each rejected attempt, then apply a valid candidate and follow any restart instruction.

Expected: administrator username, password verifier, resolved database password, tokens, and environment values never appear; each invalid candidate identifies a safe field error and leaves the prior YAML/runtime configuration byte-for-byte or semantically active; a valid candidate is atomically durable before success and reports restart requirements.

## CLI Composition Scenario

Feed multiple requests through one process so stream authentication can authorize protected operations without emitting a token:

```bash
printf '%s\n' \
  '{"version":"v1","id":"list-1","operation":"bookmark.list","payload":{"page":1}}' \
  '{"version":"v1","id":"login-1","operation":"auth.sign_in","payload":{"username":"admin","password":"<fixture-password>"}}' \
  '{"version":"v1","id":"config-1","operation":"configuration.get","payload":{}}' \
  '{"version":"v1","id":"logout-1","operation":"auth.sign_out","payload":{}}' \
  | go run ./cmd/bookmarker-cli > /tmp/bookmarker-cli.out 2> /tmp/bookmarker-cli.err
```

Expected: stdout has exactly four parseable lines in input order; configuration is redacted; no session token is returned; stderr contains only parseable safe diagnostics. Substitute the documented fixture password locally rather than committing it to a script. Remove both temporary files after inspection.

## Cross-Browser Acceptance

Run the Playwright suite, allowing its configuration to start an isolated server and fixture database when supported:

```bash
npx playwright test
```

Expected: all four user-story journeys pass in Chromium, Firefox, and the configured mobile Chromium viewport. Tests cover no-JavaScript direct navigation as well as htmx enhancement, keyboard focus, menu access, confirmation flows, responsive text containment, screenshots/placeholders, and protected direct requests.

For focused diagnosis:

```bash
npx playwright test --project=chromium --trace=on
npx playwright show-report
```

Trace, screenshot, and report artifacts must not contain production credentials or resolved secrets.

## Full Feature Gate

```bash
go test ./...
go test -tags=contract ./tests/contract/...
go test -tags=integration ./tests/integration/...
go vet ./...
npx playwright test
```

The feature is ready for approval only when all commands pass, the reviewed red-test evidence exists, no sensitive values appear in captured output, and the measurable outcomes in [spec.md](spec.md) have corresponding acceptance evidence.

Stop the manual database when finished:

```bash
docker stop bookmarker-postgres
```
