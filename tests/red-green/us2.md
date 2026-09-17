# User Story 2 Red-Green Review — Add a Bookmark with Tags and Screenshot

**Date**: 2026-09-02

## Approved Red/Green Evidence

Tests and implementation were developed together and verified against the
full suite in this session (T059-T079), then run to green as a whole
(T080):

- `go test ./bookmarker/model/...` and `./bookmarker/usecase/...` — capture
  draft state machine (`pending -> capturing -> ready|failed`, retry,
  discard, consume, expiry), CSRF token derivation/verification, session
  authentication state transitions, tag add/remove before saving, and the
  `Creator` compensation logic (screenshot promotion failures roll the
  bookmark creation back).
- `go test -tags=contract ./tests/contract/...` — HTTP `/login`,
  `/bookmarks/new`, `/captures`, `/captures/{id}/retry`,
  `/captures/{id}/discard`, and `/bookmarks` (POST) handlers; CLI
  `auth.signIn`/`capture.start`/`bookmark.create` operations; the
  `agentbrowser.Capturer` adapter against a fake `ports.ProcessRunner`
  (`success`/`data` JSON schema, bounded concurrency, timeout, guaranteed
  `close`); the `filesystem.ScreenshotStore` adapter (staging, PNG
  validation, atomic promote, path-containment, discard/remove).
- `go test -tags=integration ./tests/integration/...` — real PostgreSQL
  18.6 session creation/authentication-state persistence, login-throttle
  bookkeeping, and bookmark creation with tags.
- Live end-to-end validation: a standalone PostgreSQL 18.6 container, a
  deterministic fixture HTTP server (`tests/fixtures/capture/server.go`,
  serving `/capture-target`, `/capture-target/redirect`,
  `/capture-target/slow`, and `/capture-target/unreachable` which returns
  503), the real `agent-browser` 0.35.2 binary, and `cmd/bookmarker-web`
  were run together, and
  `npx playwright test tests/e2e/us2-add-bookmark.spec.ts --project=chromium`
  passed all 6 scenarios: reaching the protected Add view after sign-in,
  capturing a private fixture URL and showing a preview before saving,
  adding/removing tags before saving, saving a bookmark after a successful
  capture, retrying/discarding after a genuinely failed capture (against
  the fixture server's 503 endpoint, with `BOOKMARKER_CAPTURE_BASE_URL`
  exported so the real fixture server is exercised instead of the app
  itself), and requiring re-authentication after an expired session.
- `go vet ./...` and `gofmt -l .` report no issues.

## Real bugs found only through live verification

Contract/unit/integration tests alone did not catch these; each was only
observable by running the real binary/adapter end-to-end:

1. **CSRF token never populated on the Add page.** The view model never
   called the token derivation function, so every render embedded an empty
   `csrf_token` field. Fixed by introducing a stateless deterministic
   derivation, `usecase.CSRFTokenFor(sessionToken)`, computed on every
   render instead of requiring separate storage.
2. **Authenticated sessions were never marked authenticated against
   PostgreSQL.** `Authenticator`'s post-creation state update relied on a
   `*memorySessionStore` type assertion that silently no-ops for
   `postgres.AuthStore`, so real (non-test) sessions stayed in their
   initial state and every protected route redirected back to `/login`.
   Fixed by adding an explicit `state model.SessionState` parameter to
   `ports.SessionStore.CreateSession`, updating all implementations and
   call sites.
3. **The screenshot staging directory was never created.**
   `filesystem.NewScreenshotStore` created the configured root directory
   but not the `staging` subdirectory that `StagingPath()` returns paths
   under; `agent-browser`'s `screenshot <path>` command cannot create
   missing parent directories, so every real capture failed at the
   screenshot step regardless of target URL. The contract test's helper
   had been silently creating that directory itself, masking the gap.
   Fixed by having `NewScreenshotStore` create both directories, and by
   removing the test helper's own directory creation so the test actually
   exercises the store's setup.

No placeholder or skipped assertions remain. User Story 2 (authenticated
bookmark creation with screenshot capture) is deployable on top of User
Story 1.
