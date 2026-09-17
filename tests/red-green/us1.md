# User Story 1 Red-Green Review — Browse and Find Bookmarks

**Date**: 2026-09-01

## Approved Red/Green Evidence

Tests and implementation were developed together and verified against the
full suite in this session (T041-T057), then run to green as a whole
(T058):

- `go test ./bookmarker/model/...` — Unicode-aware tag normalization,
  deterministic normalized-name ordering, URL validation, all-word
  description splitting, page clamping, and bounded first/adjacent/
  selected/final pagination links with ellipsis.
- `go test ./bookmarker/usecase/...` — public browse/search orchestration,
  exact tag filtering, AND-combined criteria, cap-before-pagination,
  empty-collection handling, and the `search_criteria_required` validation
  error.
- `go test -tags=integration ./tests/integration/... -run
  TestPostgreSQLBookmarkQueries...` — real PostgreSQL 18.6 duplicate-URL
  preservation, tag normalization uniqueness, `(created_at, id)` ordering,
  `simple` text-search AND semantics, cap-before-pagination, and page
  clamping.
- `go test -tags=contract ./tests/contract/... -run
  'Root|Bookmarks|Search|CLIBookmark'` — HTTP `/`, `/bookmarks`, `/search`
  routes: field order, external-link safety (`target="_blank"
  rel="noopener noreferrer"`), tag links, empty state, full-page/fragment
  parity with `Vary: HX-Request`, and explicit-empty-search 422; CLI
  `bookmark.list`/`bookmark.search` payloads, pagination metadata,
  duplicate URLs, and cap reporting.
- Live end-to-end validation: a standalone PostgreSQL 18.6 container was
  migrated and seeded with `tests/fixtures/bookmarks.sql` (500 bookmarks,
  duplicate URLs, mixed-case tag spellings, no screenshots), `cmd/
  bookmarker-web` was run against it, and
  `npx playwright test tests/e2e/us1-browse-search.spec.ts --project=chromium`
  passed all 7 scenarios: list field order, exact tag filtering, combined
  tag/word search, 50-page pagination reachability, direct page-3 URL
  loading, htmx fragment navigation (`Vary: HX-Request`), and a mobile
  viewport.
- `go vet ./...` and `gofmt -l .` report no issues.

No placeholder or skipped assertions remain. User Story 1 (public browsing
and finding) is deployable independently as the MVP without authentication.
