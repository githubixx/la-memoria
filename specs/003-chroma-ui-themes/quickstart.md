# Quickstart: Validation Guide

This guide describes validation **after implementation**; these planned tests and commands do not assert that the feature is already implemented. See [web contract](contracts/web.md), [CLI contract](contracts/cli.md), and [data model](data-model.md).

## Prerequisites

- Go 1.27, Node.js/npm, Docker with Compose, PostgreSQL client tools (`psql`, `pg_isready`), and installed Playwright Chromium/Firefox.
- From repository root: `go mod download`, `npm ci`, `npx playwright install chromium firefox`. Docker must be running; Playwright's global setup starts its own PostgreSQL container and fixture web server. Do not point tests at a production configuration/database.
- Before feature implementation, write/review tests, run them to confirm the new cases fail, and record red-phase evidence in `tests/red-green/`; rerun after implementation and record green results.

## Automated checks

```sh
go test ./bookmarker/...
go test -tags=contract ./tests/contract/...
go test -tags=integration ./tests/integration/...
go vet ./...
npx playwright test
```

Expected: all suites pass. Add focused tests to these existing suites for:

1. Enumerating the actual pinned Chroma catalog (not a hard-coded list), unique accessible options, valid and invalid membership, absent System default and unavailable-ID recovery.
2. Real YAML/JSONL/web boundaries: omitted vs explicit invalid theme, full-candidate atomicity, permissions, secret redaction, CSRF/auth, writer short writes and injected pre-/post-rename failures. Verify cross-process writers cannot lose each other's changes.
3. Normal and htmx saves, error rendering, immediate whole-page appearance, a second visitor's full load, System light/dark and live preference change, plus database restart notice. First test-drive an isolated server fixture with its own configuration and database, managed stop/start and readiness, and cleanup; do not invoke the existing reset fixture between save and restart. For every catalog theme and System, save, reload, restart the application with the same saved configuration, and verify both the appearance and saved selection.
4. For every actual catalog theme and System, verify by keyboard alone that every primary navigation item and Configuration control is reachable, visibly focused, labeled and operable; also cover desktop/mobile primary flows. Block optional images/fonts and confirm readability and usable controls. Retain all existing browse, search, login, add/edit/delete/capture and configuration regressions.
5. Compose integration: export an old container's non-default configuration before restart or recreation, import the read-only export into an empty isolated volume, and verify all non-credential settings survive while deployment credentials refresh. Remove the one-time import setting and mount, then save theme and another setting, restart and recreate the container, and verify both settings; rotate credentials and verify settings remain. Check fresh initialization without import; missing, unreadable and invalid explicit imports; safe retry after failure; refusal to overwrite an initialized volume; owner-only permissions; and absence of secrets in logs. Use isolated test volumes and clean them up afterwards.

## Manual acceptance and user review

1. Start a non-production instance following [README setup](../../README.md), with a valid owner-only `BOOKMARKER_CONFIG`, PostgreSQL and required environment. Before upgrade omit `theme`; after startup verify light/dark System behavior.
2. Sign in, open Configuration, choose two distinct listed themes, and save each. Confirm whole-page change, selected value after reopening, a separate visitor's reload, and restart persistence. Select System and change OS/browser color preference while the page is open.
3. In an isolated test configuration set a nonexistent theme ID and restart. Confirm System fallback, visible unavailable value and blocked unrelated save; explicitly choose a valid replacement and save. Test a failure-injection environment rather than damaging production permissions.
4. Run representative primary journeys at desktop and mobile widths and with keyboard only. Capture theme-by-theme observations, including missing-artwork/font fallback.
5. Recruit volunteers who represent actual or intended Bookmarker administrators and users, aiming for varied experience and devices where feasible. Include at least one administrator for SC-001 and one user for SC-006; one person may count in both groups. Give each administrator the same unaided task and measure their first attempt from the task prompt through successful save. Ask each user to try the available themes and report whether the interface is readable and navigation and controls are easy to identify. Record recruitment method, separate group sizes, task instructions, elapsed times and responses without secrets. Calculate SC-001 as successful administrators divided by participating administrators (target ≥90%) and SC-006 as positive users divided by participating users (target ≥80%); mark a criterion unverified if its minimum is unmet. Report the limited generalizability of small samples; automated tests alone cannot establish these outcomes.

Do not delete persistent production volumes or commit real configuration files. Phase 1 ends at design; implementation and these validation runs are future work.
