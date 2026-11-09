# Bug Fix: Bookmark tags are lost when editing

- **Slug**: bookmark-tags-lost
- **Fixed**: 2026-11-02
- **Assessment**: ./assessment.md
- **Status**: applied

## Summary

PostgreSQL maintenance lookups now hydrate existing tags before rendering the edit form. Tag removal works for both initially rendered and newly added tags, while an empty submitted tag list still intentionally clears all associations.

## Changes

| File | Change | Notes |
|------|--------|-------|
| `adapters/postgres/bookmark_maintain.go` | modified | Loads display and normalized tag names ordered by normalized name; propagates query, scan, and iteration errors. |
| `web/static/js/bookmark-form.js` | modified | Delegates removal from each initialized tag list, including forms inserted by HTMX. |
| `web/templates/pages/edit.html` | modified | Marks rendered Remove buttons for the delegated handler. |
| `tests/integration/postgres_bookmark_maintain_test.go` | modified | Tests ordered hydration, untagged bookmarks, and lookup error propagation; retains replacement test. |
| `tests/contract/http_maintain_test.go` | modified | Tests escaped hidden tag fields and multiple-tag and empty replacement submissions. |
| `tests/e2e/us3-maintain-bookmarks.spec.ts` | modified | Adds a persisted preservation, filtering, adding, removal, and clear-all journey across direct and HTMX-swapped forms. |

## Tests Added or Updated

- `TestPostgreSQLBookmarkMaintainFindHydratesTags` — tag names and order, empty set, and failure when tag lookup is unavailable.
- `TestHTTPMaintainRendersAndReplacesTags` — escaped initial fields and complete-replacement POSTs, including explicit clearing.
- `editing preserves, adds and removes persisted tags on direct and swapped forms` — browser-level preservation, filtering, additions, and removals.

## Local Verification

- `TESTCONTAINERS_RYUK_DISABLED=true go test -tags=integration ./tests/integration -run 'TestPostgreSQLBookmarkMaintain' -count=1 -v` → passed using locally cached PostgreSQL image.
- `go test -tags=contract ./tests/contract -run TestHTTPMaintain -count=1 -v` → passed.
- `go test ./bookmarker/... ./adapters/postgres/...` → passed.
- `npx --no-install tsc --noEmit` → passed.
- `node --check web/static/js/bookmark-form.js` and `git diff --check` → passed.
- Isolated Chromium smoke test using `node -e` and Playwright → passed initial and added tag removal, safe DOM text, simulated HTMX after-swap initialization, and repeated initialization.
- The new full browser journey was not run: the existing Playwright fixture deletes and recreates `.e2e-config.yaml`, `.e2e-config.template.yaml`, and `.e2e-screenshots`, all of which already existed in the workspace. The isolated smoke test does not verify a real HTMX request or end-to-end persistence.

## Deviations from Assessment

None in implementation scope. Full end-to-end verification remains outstanding for the fixture-safety reason above.

## Follow-ups

- Run the new Playwright journey in an isolated workspace with disposable `.e2e-*` fixture paths.
- Previously lost associations require backup or historical evidence for recovery; this fix does not reconstruct them.
- Concurrent full-replacement edits can still overwrite one another; consider optimistic concurrency separately.
