# Chroma UI themes: red-green evidence

## Setup

- The planned dependency is `github.com/githubixx/chroma-ui v0.2.0`, tag
  commit `0de01b811e64e5814376f0b03a46dee23f7a9c30`.
- Inspection of that version's `catalog.go` and `http.go` confirms that
  `NewCatalog(nil)` selects only bundled themes, `Themes` returns metadata
  sorted by ID, `Theme` performs exact lookup, and `Handler` serves the
  allowed asset paths under a stripped prefix.
- The dependency's source carries GPL-3.0-or-later notices. Its README
  documents the vdradmin-go origin of themes, asset attribution, and artwork
  provenance; its LICENSE and notices must be retained and linked in release
  documentation.

## Foundational tests: partial red evidence

The new tests were checked against the data model and contracts: the YAML
case discovers a real bundled ID, checks the System default and retains an
obsolete ID; the write case injects a successful short write and compares
the old and resulting bytes. These are preliminary checks, not a completed
review of T003–T006. The full T007 gate has not been met.

- `go test -tags=contract ./tests/contract/... -run '^TestLoadThemeDefaultsAndPreservesStoredValues$' -count=1 -v`
  fails: the redacted type has no Theme field and strict YAML rejects `theme`.
- `go test -tags=contract ./tests/contract/... -run '^TestConfigurationActivationRejectsShortWriteWithoutChangingFile$' -count=1 -v`
  fails: activation returns success and the active bytes change after the
  injected short write.
- Initial attempts to run these tests failed because of test syntax errors;
  those were corrected before the feature-red results above. Syntax failures
  are not counted as feature evidence.
- `go mod verify` passes.

## Baseline blocker

- `go test ./bookmarker/...` fails in
  `TestCapturePreviewReadsReadyDraftScreenshot`: the test expects `conflict`
  for a failed draft, but receives `not_found`. The failure reproduces with
  `go test ./bookmarker/usecase -run '^TestCapturePreviewReadsReadyDraftScreenshot$' -count=1 -v`.
  This is unrelated to the theme tests and predates implementation changes;
  it is not counted as red evidence for this feature.
- Resolved separately: previewing a non-ready capture now reports the
  invalid-state conflict, while a ready draft without a staged image still
  reports not found. The expanded invalid-state tests failed before the fix;
  `go test ./bookmarker/...` and the focused HTTP capture lifecycle contract
  test pass afterward.
- At that point T003, T004, T005 and T006 still needed full planned coverage
  and review; no foundation implementation or green gate was claimed.

## Continued foundation test work

- T004 review: the YAML tests derive an actual bundled ID; cover missing,
  explicit System, valid and obsolete values; check redaction and secret
  resolution; and exercise strict fields and unsafe permissions with a theme
  present. The strict-field test first requires a valid theme to load, so an
  unknown `theme` field cannot produce a false positive.
- `go test -tags=contract ./tests/contract/... -run '^TestLoadTheme' -count=1 -v`
  fails for the intended missing Theme property and rejected YAML field.
  The owner-only permission subtest passes. This is T004 red evidence, not the
  complete T007 gate.
- `go test ./bookmarker/...` passes; the previously noted capture baseline
  failure is resolved. Docker reports server version 29.9.1.
- T003, T005 and T006 still need full planned coverage and review.
- T003 partial progress: the pinned upstream catalog is nonempty, ordered,
  unique, case-sensitive and does not include System. The new service
  projection cases test both a missing setting and an obsolete value.
  `go test ./bookmarker/usecase -run '^(TestBundledThemeCatalogHasSortedUniqueExactIDs|TestConfigurationServiceGetProjectsDefaultAndUnavailableTheme)$' -count=1 -v`
  passes the upstream catalog test and fails both projection cases because
  the model has no Theme field. Catalog adapter/port behavior and
  complete-candidate, membership and presence validation still need tests;
  T003 and the T007 gate remain incomplete.
- T005 partial progress: added a real staged-file sync failure by closing the
  staged descriptor after writing. The test verifies unchanged active bytes,
  staged-file cleanup and owner-only mode. In
  `go test -tags=contract ./tests/contract/... -run '^TestConfigurationActivation(PreservesFileWhenStagedSyncFails|RejectsShortWriteWithoutChangingFile)$' -count=1 -v`,
  the sync test passes and the short-write test fails because the old file
  is replaced. Close, directory-open, rename and post-rename sync injection
  still need coverage; T005 and T007 remain incomplete. An intermediate test
  placement produced a Go syntax error; it was corrected before the results
  above and is not feature-red evidence.

## Foundation red gate (T003–T007)

- Reviewed T003 for catalog ordering, uniqueness and exact membership, System,
  missing and obsolete stored values, omission and explicit invalid selection,
  validate/update, and rejection of an invalid complete candidate. The tests
  derive a real catalog ID and check that rejected updates do not activate.
- Reviewed T005 for injected short writes and real staged-sync failure, plus
  staged sync/close, directory open, rename and post-rename directory sync
  failures. Pre-commit checks compare exact bytes and check staging cleanup and
  permissions; the committed case expects a redacted warning and success.
- Reviewed T006: separate test processes acquire a shared transaction lock,
  modify distinct fields and read the actual YAML file while writes occur.
  Separate timeout and callback-failure checks cover bounded acquisition and
  release. T004 review is recorded above.
- Scope of that concurrency result: each writer loads the current configuration
  inside its locked transaction before changing its field. The update API
  accepts full configuration candidates and does not carry a baseline or
  revision; a stale, independently prepared full candidate can overwrite
  another writer's changes to fields it includes. The test does not establish
  automatic merging or conflict detection for such stale submissions.
- `go test ./bookmarker/usecase -run 'Test(ConfigurationThemeCandidateValidationAndPresence|ThemeValidationRejectsEntireInvalidCandidate|BundledThemeCatalogHasSortedUniqueExactIDs|ConfigurationServiceGetProjectsDefaultAndUnavailableTheme)' -count=1 -v`
  fails because the planned `adapters/chromatheme` package does not exist.
- `go test -tags=contract ./tests/contract/... -run '^Test(LoadTheme|ConfigurationActivation)' -count=1 -v`
  fails because the planned filesystem injection boundary does not exist.
  Earlier focused runs above independently demonstrated the missing YAML
  behavior and the real short-write defect.
- `go test -tags=integration ./tests/integration/... -run '^TestConfiguration(CrossProcessTransactionsAndReaders|LockIsBoundedAndReleasedOnFailure)' -count=1 -v`
  fails because the planned cross-process `WithLock` API does not exist.
- An initial placement of two added test functions caused syntax failures;
  that was corrected and formatted before the red commands above. Syntax
  failures are not counted as feature evidence.

## Foundation green gate (T014)

- `go test ./bookmarker/... -count=1` passes, including the theme candidate
  and capture baseline cases.
- `go test -tags=contract ./tests/contract/... -run '^Test(LoadTheme|ConfigurationActivation|ConfigurationStore)' -count=1 -v`
  passes, including all injected failure points and short writes.
- `go test -tags=integration ./tests/integration/... -run '^TestConfiguration(CrossProcessTransactionsAndReaders|LockIsBoundedAndReleasedOnFailure)' -count=1 -v`
  passes for two processes, concurrent reads, deadline and lock release.
- Refactoring: extracted the candidate validation path to avoid a second
  load/apply on update; the commit point is explicit, and the warning does not
  contain configuration values. Formatted the modified Go files.

## US1 tests in progress

- T015 reviews actual mounted catalog assets, GET/HEAD, ETag revalidation,
  forbidden metadata and unsupported methods. T016 exercises missing,
  unavailable, valid and failed-read appearance, including redacted logging.
- `go test -tags=contract ./tests/contract/... -run '^Test(ChromaAssetsAreAllowlistedAndRevalidate|PublicShellUsesOnlyApprovedAppearance)$' -count=1 -v`
  fails because the planned HTTP assets and appearance projection APIs are
  absent.
- `npx tsc --noEmit` passes for the new journey tests. Running the Chromium
  System-shell case in an isolated temporary copy on ports 25329/28080/28081
  fails as intended: the Chroma base stylesheet is missing. This copy was
  necessary because the original fixture ports were already occupied and the
  existing setup overwrites/deletes local fixture files. The temporary fixture
  used its own database container and cleaned it up.
- The new journeys cover browsing, search, sign-in, maintenance, capture,
  configuration, live color preference, blocked decorative assets, labeled
  controls, keyboard focus and the restart feedback notice.
- Reviewed T017 and ran the complete new file in the isolated copy against
  Chromium, Firefox and mobile Chromium. Five tests pass; the System shell
  case fails in all three projects on the missing Chroma base CSS, as expected.
  The mobile capture case additionally fails because the save button is
  intercepted by other content; this is a real responsive regression to
  address before the green gate, not evidence of the missing stylesheet.

## US1 green gate (T027)

- `go test -tags=contract ./tests/contract/... -run '^Test(ChromaAssetsAreAllowlistedAndRevalidate|PublicShellUsesOnlyApprovedAppearance|Bookmarks|Search|Root)' -count=1 -v`
  passes. `go test ./bookmarker/... -count=1` and both command package
  builds pass.
- The new isolated browser journey file passes all 9 cases across Chromium,
  Firefox and mobile Chromium, including the formerly obstructed mobile
  capture save and live System changes.
- The five existing browse, create, maintenance, configuration and
  accessibility browser files initially produced 72 passes and 9 responsive
  menu failures. Removing inherited Chroma nav padding restored viewport
  containment. A focused rerun then exposed Firefox fractional rounding at
  exactly 44px; setting a 45px minimum corrected that. All 9 responsive
  cases subsequently passed across the three browser projects.
- The full Go contract suite initially exposed an existing logger redaction
  regression for plural cookies and form bodies. The existing failing
  redaction test drove adding those exact attribute names; the full contract
  suite then passed. No secret values were recorded here.
- Chroma's base and theme CSS import remote fonts; a same-origin style/font
  policy blocks those requests while retaining system font fallback. Browser
  tests also block artwork to exercise graceful fallback.

## US2 red tests in progress

- T028: real JSONL stream tests cover validate and update with omission,
  default, obsolete stored value, explicit null, empty, wrong type, unknown,
  System and real catalog IDs, asserting validation envelopes and unchanged
  state on rejection. T029 tests actual protected form requests, missing,
  empty and tampered values through normal and htmx, complete-update
  preservation, catalog options and restart redirects.
- `go test -tags=contract ./tests/contract/... -run '^Test(CLIThemePresenceValidationAndAtomicity|HTTPThemeSelectionAndInvalidSubmissions)$' -count=1 -v`
  fails: JSONL ignores theme presence and form options are missing.
  Running the HTTP test alone confirms that all missing/empty/tampered
  normal and htmx submissions currently succeed and modify state, as expected
  for the feature-red gate.
- T053 reviewed before T054: tests check unchanged saved bytes across actual
  stop/start, HTTP readiness, surfaced startup/termination errors and cleanup
  of fixture-owned directories. The isolated Chromium run of
  `npx playwright test tests/e2e/restart-fixture.spec.ts --project=chromium`
  fails because the planned `restart-fixture` module does not exist.
- T054 green: `npx tsc --noEmit` passes; the three restart-fixture tests pass
  under Chromium in the isolated copy. The fixture owns a temporary
  configuration, PostgreSQL container, free ports and compiled web process;
  it does not call `resetE2E` and removes only its own resources.
- T030 uses the pinned catalog directly to derive all 37 IDs plus System and
  the verified restart fixture. Its isolated Chromium red run reaches the
  configuration screen and fails because no Theme options are rendered.
  The planned loop also checks second-visitor appearance, reload, restart,
  live System behavior and keyboard reachability/focus/labels.
- T031 reviews real isolated filesystem cases for failed import, retry,
  credential rotation, refusal to overwrite and permissions, alongside
  isolated Compose-volume export/import, restart/recreation and invalid
  import tests. The test helper now assigns a unique configuration volume
  name, so cleanup never targets a production volume.
- `go test -tags=integration ./tests/integration/... -run '^TestCompose(InitializerImportFailuresRetryAndRotation|ConfigExportImportRestartAndRecreation)$' -count=1 -v`
  fails to build because the planned initializer API does not exist. This is
  feature-red evidence; no Compose lifecycle success is claimed yet.
- T032 review complete: JSONL null/empty/wrong-type/unknown cases fail for
  presence handling, omitted obsolete cases already pass at the shared
  foundation, and both normal and htmx missing/tampered form submissions
  incorrectly save changes before T034. Initial test-placement syntax errors
  were corrected before these red results and are not counted as evidence.

## US2 green gate (T038)

- Focused CLI and HTTP configuration contract suites pass, including null,
  empty, wrong-type, case-mismatched and unknown IDs; omitted obsolete ID;
  and missing/tampered ordinary and htmx form submissions. The full contract
  suite passes after updating its old shell-specific permission assertion to
  recognize the new initializer; atomic 0600 permissions are exercised at
  the filesystem and Compose boundaries instead.
- The isolated Chromium selection test passes for all 37 bundled themes and
  System, including per-theme saves, second visitor, reload, process restart,
  keyboard operation and System preference changes. A first run found that
  the Mantle stylesheet suppressed a focused control's outline; compatibility
  CSS now restores visible focus. Another run exposed that the fixture's
  five-second termination timeout was shorter than the server's graceful
  shutdown window; the fixture now allows 15 seconds and the browser cancels
  in-flight decorative requests before stopping. The fixture's own three
  tests were rerun green after that change.
- `go test -tags=integration ./tests/integration/... -run '^TestComposeInitializerImportFailuresRetryAndRotation$' -count=1 -v`
  passes. The isolated Compose failure/import test passes for missing,
  invalid and unreadable imports, safe retry and reimport refusal. The
  export/import lifecycle test passes with preserved settings and theme
  across recreation and restart, owner-only mode and credential rotation.
  The containers and named volumes used unique test-owned names and were
  cleaned up.

## US3 red gate (T039–T042)

- Reviewed the CLI test against a real owner-only YAML file: obsolete
  omission and combined invalid changes reject with the existing validation
  envelope and preserve exact bytes; an explicit valid replacement succeeds.
  This test already passes because Phase 2 implemented the shared invariant.
- Reviewed HTTP tests for escaped unavailable selection, invalid combined
  updates and injected writer failures, ordinary and htmx 422, candidate
  preservation, persisted shell, no-store and secret redaction.
  `go test -tags=contract ./tests/contract/... -run '^Test(CLIRecoversObsoleteThemeWithoutPartialUpdates|HTTPThemeRecoveryAndFailurePresentation)$' -count=1 -v`
  fails on the missing unavailable sentinel; the already-implemented
  state-preservation cases pass.
- The isolated Chromium recovery test fails before implementation because
  an obsolete value silently displays System instead of the selected
  unavailable sentinel. Its subsequent steps cover explicit recovery,
  an actual bounded lock-contention save failure, accessible error
  association, htmx form display and an ordinary tampered submission.
  A temporary TypeScript matcher error was corrected before this run and
  is not counted as feature-red evidence.

## US3 green gate (T047)

- Focused CLI, HTTP and atomic activation contract tests pass, including
  real-file obsolete recovery, short-write byte preservation, invalid form
  requests and pre-/post-commit failure injection.
- The isolated recovery browser test passes in Chromium, Firefox and mobile
  Chromium. It verifies escaped unavailable selection, blocked unrelated
  save, explicit replacement, accessible error association, htmx 422 swap
  under real lock contention, a normal tampered submission and unchanged
  saved bytes/appearance.
- The htmx error swap is restricted to the configuration form and its content
  target. Ordinary error pages get their appearance from the last persisted
  configuration, while candidate inputs remain in the form.

## Acceptance and participant review status (T052)

- Automated isolated browser coverage exercises the acceptance journeys and
  all catalog themes; it is not a substitute for human manual review.
- No administrator or user volunteers have been recruited in this session.
  Administrator sample size: 0; user sample size: 0. No first-attempt
  timings or participant opinions are available. SC-001 and SC-006 remain
  **unverified**, not passed.
- Completing T052 requires recruiting at least one administrator and one
  user (one person may serve both roles), conducting the prescribed isolated
  manual tasks, recording theme-by-theme observations and first-attempt
  times, and reporting the very limited generalizability of small samples.

## Full verification progress (T051)

- The first `go test -tags=integration ./tests/integration/... -count=1
  -timeout=40m` run failed two pre-existing no-service-environment Compose
  scenarios: the helper required its isolation-only `.compose.env` before
  those tests wrote it. The helper now creates port/volume interpolation
  settings when copying every stack, and removes only the copied `.env` so
  no-credentials tests cannot inherit a developer's runtime settings.
- The two affected integration tests then passed, followed by the full
  integration suite: `go test -tags=integration ./tests/integration/...
  -count=1 -timeout=40m` (124.032s).
- `go test ./bookmarker/... -count=1` passed.
- `go test -tags=contract ./tests/contract/... -count=1` passed.
- `go vet ./...` and `git diff --check` passed.
- `npx playwright test` passed all 117 tests across Chromium, Firefox and
  mobile Chromium (17.6 minutes). It ran in an isolated checkout with
  dedicated ports and its own database, rather than overwriting the existing
  checkout's live fixture and runtime files.
