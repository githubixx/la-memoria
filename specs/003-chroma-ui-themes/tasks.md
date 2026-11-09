# Tasks: Chroma UI and Theme Selection

**Input**: `specs/003-chroma-ui-themes/{spec.md,plan.md,research.md,data-model.md,contracts/,quickstart.md}`

**Organization**: Setup, shared prerequisites, then independently verifiable user-story increments in priority order. Test-first is mandatory under the constitution and explicitly required by the quickstart. Every red gate requires reviewed tests and recorded failing evidence before the corresponding implementation; run the same tests green and refactor afterward.

**Format**: `- [ ] Tnnn [P?] [USn?] action with path`. `[P]` indicates tasks in the same explicitly named parallel group touch different files and have no unmet dependencies; it does not authorize parallel edits to the same file across phases.

## Phase 1: Setup

- [X] T001 Pin `github.com/githubixx/chroma-ui` to `v0.2.0` in `go.mod` and update `go.sum`; verify the bundled catalog API and asset license notices against that version without enabling overrides.
- [X] T002 Prepare the feature-specific red/green evidence log at `tests/red-green/chroma-ui-themes.md`, recording test commands, review, expected failures, later green results and any environmental blockers without secrets.

## Phase 2: Foundational — shared catalog and safe configuration transactions

**Purpose**: Provide a single catalog, compatible configuration data, strict update semantics and safe persistence before any story implementation. The tests below precede the shared implementation.

### Tests first

- [X] T003 [P] Add catalog and configuration unit cases in `bookmarker/usecase/configuration_test.go` for sorted unique catalog IDs, System, missing/obsolete stored values, case-sensitive membership, complete-candidate validation, and omission versus explicit invalid input.
- [X] T004 [P] Extend `tests/contract/config_load_test.go` for missing, valid and obsolete YAML themes, strict known fields, owner-only permissions and unchanged secret handling.
- [X] T005 [P] Extend `tests/contract/config_update_test.go` with real filesystem and injected short-write, staged-sync/close, directory-open, rename and post-rename directory-sync failures; assert prior bytes on every pre-commit error, success plus warning after commit, and restrictive permissions.
- [X] T006 [P] Add cross-process writer/reader cases in `tests/integration/configuration_concurrency_test.go` using a shared config and stable sibling lock: concurrent distinct changes survive, reads are complete, lock acquisition is bounded, and failures release the lock.
- [X] T007 Review and run T003–T006 before implementation and record failing assertions or missing-API build failures, commands and review in `tests/red-green/chroma-ui-themes.md`; resolve infrastructure failures separately rather than counting them as feature-red evidence.

### Implementation

- [X] T008 Define theme option/catalog and transaction-lock boundaries in `bookmarker/ports/config.go` and `bookmarker/ports/theme.go`: catalog metadata and exact membership, plus a bounded cross-process transaction covering load through commit.
- [X] T009 Implement the immutable embedded Chroma adapter in `adapters/chromatheme/catalog.go`, using `chromaui.NewCatalog(nil)`, sorted bundled metadata, safe lookup and handler access; surface catalog-construction errors rather than silently continuing.
- [X] T010 Extend `bookmarker/model/config.go` and `bookmarker/model/configuration_edit.go` with YAML theme, redacted theme, System default and explicit theme-presence representation; allow obsolete values to load but never coerce explicit invalid submissions.
- [X] T011 Update `adapters/configyaml/load.go` to accept absent and unavailable stored themes while retaining strict YAML parsing and existing credential handling.
- [X] T012 Implement a bounded stable sibling-file lock and atomic staged write in `adapters/configyaml/update.go`: check short writes, sync/close and set 0600 before rename, open the directory pre-rename, clean up/unlock on all paths, and log post-rename directory-sync failure as a redacted durability warning rather than a failed save.
- [X] T013 Update `bookmarker/usecase/configuration.go` to merge omitted theme under the transaction lock, reject explicit empty/unknown or retained obsolete IDs, validate the entire candidate, commit once under the same lock, and retain database restart and redaction behavior; keep validate read-only. JSON null presence remains a T033 boundary responsibility.
- [X] T014 Run the focused T003–T006 suites and record green results and refactoring evidence in `tests/red-green/chroma-ui-themes.md`; do not advance with unexplained failures.

## Phase 3: User Story 1 — Use the refreshed interface (P1) 🎯 MVP

**Goal**: Preserve all public and administrator journeys using Chroma's shared foundation, with System as the default and secure rendering of persisted appearance.

**Independent test**: With a pre-feature configuration lacking `theme`, navigate, sign in, browse/search and perform authorized bookmark/configuration operations at desktop and mobile widths; verify keyboard controls and live System light/dark behavior.

### Tests first

- [X] T015 [P] [US1] Add `tests/contract/chroma_assets_test.go` to exercise the actual catalog handler at `/ui/`, GET/HEAD allowlisting, prefix stripping, content types, cache revalidation, invalid paths and metadata non-disclosure.
- [X] T016 [P] [US1] Extend `tests/contract/http_public_test.go` to verify full-page System appearance for absent/obsolete configuration, safe URLs for valid catalog IDs, and redacted logged fallback on read failure.
- [X] T017 [P] [US1] Add `tests/e2e/chroma-journeys.spec.ts` covering desktop/mobile visitor and administrator sign-in, browse, search, add, edit, delete, capture, configuration and feedback, keyboard focus/labels, blocked fonts/artwork and live System preference changes.
- [X] T018 [US1] Review and run T015–T017 before story implementation; record red results and environment blockers in `tests/red-green/chroma-ui-themes.md`.

### Implementation

- [X] T019 [US1] Wire one Chroma catalog per process and fail startup on construction errors in `cmd/bookmarker-web/main.go` and `cmd/bookmarker-cli/main.go`, sharing it with configuration validation.
- [X] T020 [US1] Add a non-secret effective appearance projection in `bookmarker/usecase/appearance.go`: System on absent, obsolete or read failure, catalog-approved IDs only, and structured redacted read-failure logging.
- [X] T021 [US1] Integrate the projection and Chroma `/ui/` handler into `adapters/httpweb/router.go` and `adapters/httpweb/render.go`, providing only approved appearance data to each full-page shell while retaining fragment and access behavior.
- [X] T022 [US1] Update `web/templates/layouts/base.html` and `web/templates/fragments/menu.html` for Bookmarker branding, Chroma components and the documented base → layout → selected theme → compatibility CSS order; System emits no selected-theme link or explicit override.
- [X] T023 [P] [US1] Adapt `web/templates/pages/login.html`, `web/templates/pages/list.html`, `web/templates/pages/search.html` and `web/templates/fragments/bookmark_results.html` to shared tokens/components while preserving navigation, authorization, labels and empty/error states.
- [X] T024 [P] [US1] Adapt `web/templates/pages/add.html`, `web/templates/pages/edit.html`, `web/templates/pages/delete.html`, `web/templates/fragments/capture.html` and `web/templates/fragments/delete_result.html` to shared tokens/components without changing workflow or htmx semantics.
- [X] T025 [US1] Adapt `web/templates/pages/configuration.html` to Chroma components while preserving existing fields, labels, feedback and administrator restrictions; coordinate later theme-control edits with US2.
- [X] T026 [US1] Update `web/static/css/bookmarker.css` and add `web/static/css/chroma-compat.css` for shared tokens, fallback fonts/artwork, visible focus, readable feedback and responsive mobile navigation.
- [X] T027 [US1] Run T015–T017 and the existing journey regressions; record green results in `tests/red-green/chroma-ui-themes.md`.

## Phase 4: User Story 2 — Select and apply a theme (P1)

**Goal**: Administrators select System or any actual bundled theme, save one deployment-wide choice, and see it immediately and after reload or restart, including Compose recreation.

**Independent test**: Save two distinct catalog themes and System; verify the full current page and a second visitor's reload, the selected option, live device preference in System, and application/Compose restart persistence.

### Tests first

- [X] T028 [P] [US2] Extend `tests/contract/cli_configuration_test.go` using real JSONL encode/decode and the shared use case: get, validate/update, old-client omission, default System, exact IDs, authentication, unknown fields, redaction and restart indication. Before implementing theme presence handling, test explicit null, empty, wrong-type and unknown theme IDs and omission when the stored ID is obsolete through both validate and update; assert the validation error envelope and unchanged saved configuration.
- [X] T029 [P] [US2] Extend `tests/contract/http_configuration_test.go` for administrator-only catalog options exactly once, unique labels, selected value, valid normal 303 and htmx HX-Redirect, no-store, CSRF, simultaneous database changes and restart notice. Before implementing the required theme field, test missing and tampered theme submissions through both normal and htmx requests: each returns a validation error, rejects the entire update, and preserves the previously saved configuration and appearance.
- [X] T030 [P] [US2] Add `tests/e2e/chroma-selection.spec.ts` using the isolated restart fixture from T054 and deriving every theme from the actual catalog plus System; save each, verify whole-page appearance, second visitor, reload and server restart with the same saved configuration, plus live light/dark changes. For every theme and System, use keyboard-only navigation to assert that every primary navigation item and Configuration control is reachable, visibly focused, labeled and operable.
- [X] T031 [P] [US2] Extend `tests/integration/compose_setup_test.go` with isolated-volume tests that export a non-default configuration from the old container before restart/recreation, import it into an empty volume, refresh credentials, and preserve all other settings through restart and recreation. Cover fresh initialization without import, missing/unreadable/invalid requested imports without default fallback or partial initialization, retry after failed import, refusal to overwrite an initialized volume, 0600 permissions, no secret logs and cleanup without touching production volumes.
- [X] T053 [US2] Add and review `tests/e2e/restart-fixture.spec.ts` tests for an isolated server fixture: saved configuration survives stop/start unchanged, readiness is checked, startup/termination failures are surfaced, and cleanup is limited to fixture-owned resources. Run them before T054, recording expected fixture-red evidence in `tests/red-green/chroma-ui-themes.md`.
- [X] T054 [US2] Implement the test-only restart fixture in `tests/e2e/restart-fixture.ts` after T053's reviewed red gate: use an isolated temporary configuration and database, separate ports and an owned web process; stop and restart it against the same saved file without invoking `resetE2E`, wait for exit and readiness, surface failures, and clean up fixture-owned resources even on test failure. Run T053 green and record results before running T030.
- [X] T032 [US2] Review and run T028–T031 before story implementation, including T028's JSONL invalid and obsolete-theme rejection cases, T029's missing and tampered theme submission cases, and T030 with the verified T054 fixture; record feature-red results and environment blockers in `tests/red-green/chroma-ui-themes.md`. Confirm the JSONL rejection cases fail for the intended missing behavior before T033 and the web rejection cases before T034.

### Implementation

- [X] T033 [US2] Update `adapters/jsonl/configuration_operations.go` and `adapters/jsonl/protocol.go` to preserve theme property presence in v1 get/validate/update, reject explicit null, empty, wrong-type and unknown values and omitted obsolete selections with the validation error envelope, keep strict decoding and existing authentication/error envelopes, and route both operations through the shared validator without changing saved configuration on rejection.
- [X] T034 [US2] Extend `adapters/httpweb/configuration_handlers.go` to provide catalog options and saved selection, require a theme form field, reject missing or tampered values without changing saved configuration or appearance, submit the full candidate, and retain normal 303 or htmx HX-Redirect with database restart indication.
- [X] T035 [US2] Add the labeled native Theme select in `web/templates/pages/configuration.html`, displaying System and each bundled catalog option once with distinguishable names and the stored valid selection.
- [X] T036 [US2] Implement YAML-aware first-time configuration initialization in `cmd/bookmarker-config-init/main.go` and `adapters/configyaml/compose.go`: import and validate an explicitly supplied old configuration only into an empty volume, fail without default fallback on missing/unreadable/invalid imports, refuse imports into initialized volumes, and atomically refresh only validated deployment-owned administrator credentials. Preserve all other fields, locking, 0600 permissions and redacted diagnostics; initialize from the template only when no import is requested.
- [X] T037 [US2] Build and invoke the initializer from `deploy/compose/Dockerfile` and `deploy/compose/entrypoint.sh`, mount an owner-only persistent named configuration volume in `deploy/compose/docker-compose.yml`, and support one-time `BOOKMARKER_CONFIG_IMPORT_PATH` pointing to an operator-provided read-only mounted export. Leave direct `BOOKMARKER_CONFIG` deployments unchanged.
- [X] T038 [US2] Run T028–T031 and record green results in `tests/red-green/chroma-ui-themes.md`, including the per-theme restart and Compose recreation outcomes.

## Phase 5: User Story 3 — Recover safely from invalid changes (P2)

**Goal**: Explain invalid/obsolete selections and storage failures without changing the last committed settings or appearance, and allow explicit recovery.

**Independent test**: Submit an invalid theme and inject a pre-commit failure; verify errors and unchanged saved bytes/appearance. Load an obsolete ID, verify System fallback and blocked unrelated save, then explicitly replace it.

### Tests first

- [X] T039 [P] [US3] Extend `tests/contract/cli_configuration_test.go` for explicit recovery from an obsolete ID, combined invalid changes, unchanged bytes and the correct validation envelope; retain T028's earlier JSONL invalid-input and omitted-obsolete rejection tests.
- [X] T040 [P] [US3] Extend `tests/contract/http_configuration_test.go` with recovery and failure-presentation cases for missing/tampered/obsolete theme, unavailable selected sentinel, invalid other fields, writer failure, 422 full/htmx responses, preserved candidate input, persisted shell appearance, no-store and secret redaction; retain T029's earlier rejection tests.
- [X] T041 [P] [US3] Add `tests/e2e/chroma-recovery.spec.ts` covering obsolete stored ID fallback, blocked unrelated save, explicit replacement, invalid/failed normal and htmx saves, accessible error association and preserved appearance/settings.
- [X] T042 [US3] Review and run T039–T041 before story implementation; record red results and environment blockers in `tests/red-green/chroma-ui-themes.md`.

### Implementation

- [X] T043 [US3] Update `adapters/httpweb/configuration_handlers.go` to preserve candidate form data on failure, project the persisted appearance for full error pages, show actionable errors, and render an obsolete raw value as a selected unavailable sentinel requiring replacement.
- [X] T044 [US3] Update `web/templates/pages/configuration.html` with the escaped unavailable sentinel, required selection and accessible error association; ensure invalid values cannot generate asset URLs.
- [X] T045 [US3] Add narrowly scoped htmx 422 form-swap handling in `web/static/js/configuration.js` and include it from `web/templates/layouts/base.html`, preserving existing CSRF and successful full-navigation behavior.
- [X] T046 [US3] Complete recovery-specific invalid-input and storage-failure mapping in `adapters/jsonl/configuration_operations.go` and `adapters/httpweb/configuration_handlers.go`, preserving the existing validation envelope and T034's rejection behavior, and never reporting post-rename durability warnings as failed saves.
- [X] T047 [US3] Run T039–T041 and relevant failure-injection regressions; record green results in `tests/red-green/chroma-ui-themes.md`.

## Phase 6: Polish & cross-cutting validation

- [X] T048 [P] Update `README.md` and `deploy/compose/.env.example` with System/default selection, catalog licensing and attribution, persistent-volume ownership and credential rotation. Document stopping configuration changes and securely exporting the old `/run/bookmarker/config.yaml` before any restart/recreation; owner-only backup and rollback; the one-time read-only import mount and `BOOKMARKER_CONFIG_IMPORT_PATH`; removing both after successful import before restart; safe handling of failed imports and initialized volumes; and the limitation that already-lost ephemeral settings cannot be recovered.
- [X] T049 [P] Update `CHANGELOG.md` with 0.3.0 migration notes for exported Go struct/unkeyed literals and strict JSON response consumers, while documenting omitted-theme JSONL v1 compatibility.
- [X] T050 Update the Bookmarker version to 0.3.0 in `go.mod`, `package.json` and `package-lock.json`; retain third-party version numbers and record the Chroma GPL-3.0-or-later notices and bundled asset attribution in `README.md`.
- [X] T051 Run `go test ./bookmarker/...`, `go test -tags=contract ./tests/contract/...`, `go test -tags=integration ./tests/integration/...`, `go vet ./...` and `npx playwright test` per `specs/003-chroma-ui-themes/quickstart.md`; record commands, results, red/green evidence and any blockers in `tests/red-green/chroma-ui-themes.md`.
- [ ] T052 Execute the isolated manual acceptance and participant review in `specs/003-chroma-ui-themes/quickstart.md`; recruit at least one administrator and one user, with overlap permitted; record recruitment, theme-by-theme observations, separate sample sizes, first-attempt timing and SC-001/SC-006 outcomes in `tests/red-green/chroma-ui-themes.md` without secrets. Mark a criterion unverified if its minimum is unmet, and identify the limited generalizability of small samples.

## Dependencies & execution order

- Phase 1 → Phase 2 → US1 → US2 → US3 → Phase 6. US1 is the standalone System-mode MVP; US2 relies on its shared shell and appearance projection; US3 relies on US2's form and JSONL integration. US3's rejected-update guarantees are already enforced at the shared transaction boundary in Phase 2, while its phase completes recovery and failure presentation.
- Within each phase, tests and their review/red gate precede implementation; the green gate follows all implementation in that phase. Within Phase 2, T008 precedes T009/T010; T010 precedes T011 and T013; T008 and T011–T012 precede T013. T014 requires T009–T013.
- Within US1, T019–T021 precede T022; T023/T024 can proceed together after T022; T025 and T026 follow T022, with shared-file edits serialized. T027 requires T019–T026.
- Within US2, T053's reviewed red gate precedes T054; T053 must pass with the completed fixture before running T030 and the T032 feature-red gate. T033 and T034 follow T032, which explicitly includes T028's JSONL invalid-input and omitted-obsolete rejection tests and T029's missing and tampered form-submission rejection tests; T035 follows T034; T036 follows the red gate and shared locking, T037 follows T036; T038 requires T033–T037 and the T028 and T029 rejection tests to pass. Coordinate all edits to shared files sequentially.
- Within US3, T043 precedes T044; T045 follows T043 and runs separately from other edits to `web/templates/layouts/base.html`; T046 follows T043 and is serialized with it. T047 requires T043–T046.
- Polish follows completed stories: T048 and T049 may run together; T050 follows T048 because both edit `README.md`; T051 follows T050; T052 follows implementation and is independent of T051 except for shared evidence-log edits, which must be serialized.

## Parallel examples

- **Foundation**: T003, T004, T005 and T006 can be written concurrently in separate files; review and run all at T007 before coding.
- **US1**: T015, T016 and T017 are independent test files; after T022, T023 and T024 adapt separate templates in parallel.
- **US2**: T028, T029, T030, T031 and T053 can be written concurrently in separate files; T054 follows T053's red gate, and T030 runs only after the fixture passes T053. After T032, T033 and T036 can proceed independently, then T037 waits for T036.
- **US3**: T039, T040 and T041 can be written concurrently after US2; T043–T046 are serialized where they touch the same handler/template.

## Implementation strategy

1. Complete setup and the shared transaction/catalog foundation, retaining reviewed red-before-green evidence.
2. Deliver US1 alone as the MVP; independently validate the refreshed System-mode interface and existing journeys.
3. Add US2 for full deployment-wide selection, CLI compatibility and durable Compose storage; verify each actual catalog option and System.
4. Add US3 for explicit obsolete-value recovery and visible, accessible save failures; then complete cross-cutting release and acceptance checks.
5. Participant-study success criteria are not inferred from automated checks; report missing participants or infrastructure as unverified, not passed.
