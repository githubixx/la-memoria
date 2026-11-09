# Implementation Plan: Chroma UI and Theme Selection

**Branch**: `master` | **Date**: 2026-10-29 | **Spec**: [spec.md](spec.md)

## Summary

Adopt the pinned, embedded Chroma UI foundation and its actual bundled catalog throughout Bookmarker, retaining existing server-rendered workflows and htmx. Store one deployment-wide selection in YAML; `system` is the default and follows the browser's color preference. Validate all changes through the shared configuration use case, preserve omitted CLI values, and serve a safe System fallback for unavailable persisted IDs without replacing them. Make the existing configuration commit semantics and Compose persistence meet the feature's atomicity and restart requirements.

## Technical Context

**Language/Version**: Go 1.27 (module), HTML/CSS and vanilla JavaScript; TypeScript only for Playwright tests.  
**Primary Dependencies**: `github.com/githubixx/chroma-ui` `v0.3.0` (pin exact version; originally planned with `v0.2.0`), existing yaml.v3, htmx, PostgreSQL/pgx; Playwright 1.62.1.  
**Storage**: Existing owner-only YAML configuration; PostgreSQL for bookmark/auth data; no database migration. Compose needs a persistent configuration volume.  
**Testing**: Go unit, contract and integration tests; Playwright Chromium, Firefox, and mobile Chromium; manual participant review.  
**Target Platform**: Existing Linux/Compose Go server and desktop/mobile browsers.  
**Project Type**: Reusable Go library with web and JSON Lines CLI adapters.  
**Performance Goals**: No new numeric latency target in the spec; avoid runtime external asset fetching and per-request catalog discovery; apply on successful save without server restart.  
**Constraints**: Bundle only; no custom overrides; no secrets in output/logs; same-origin and CSRF protections; fallback without artwork or fonts; no live update of already-open other pages.  
**Scale/Scope**: All existing pages, fragments, authorized workflows, and every theme returned by the pinned catalog plus System. No user-specific preferences.

## Constitution Check

| Gate | Before research | Post-design |
| --- | --- | --- |
| I. Library-first, ports, use cases, adapters | PASS: extend `bookmarker` configuration model/use case; put Chroma integration at an adapter boundary. | PASS: catalog/validation port and existing store/activator isolate implementation; HTML/CSS remain delivery concerns. |
| II. Composable CLI | PASS: existing JSONL CLI is the library delivery interface. | PASS: authenticated v1 get/validate/update contract preserves absent theme and stdout/stderr separation; see [CLI contract](contracts/cli.md). |
| III. Test-first | PASS: required for implementation. | PASS: tests first, recorded red phase then green/refactor for each change; no feature implementation in this plan. |
| IV. Contract/integration | PASS: new theme, YAML and interface contracts require real boundary tests. | PASS: catalog/asset HTTP, YAML filesystem, CLI and web integration, Compose restart and Playwright acceptance planned. |
| V. Observability, simplicity, compatibility | PASS: existing structured logging, no added Repository/UoW. | PASS: structured, redacted failure diagnostics; exact dependency pin; 0.3.0 MINOR and migration notes for potentially breaking exported/JSON shape changes. |
| Browser/technology/secrets | PASS: vanilla frontend and existing stack; GPL dependency subject to attribution. | PASS: Chroma assets embedded, browser code minimal, no secrets exposed, owner-only verifier configuration retained. |

Gates pass; no unresolved architectural exceptions or clarification markers. Post-design approval still requires evidence of the mandated red-green sequence and tests. Existing Compose storage and writer defects are explicitly included, rather than assumed away.

## Project Structure

### Documentation

```text
specs/003-chroma-ui-themes/
├── spec.md
├── plan.md
├── research.md
├── data-model.md
├── contracts/
│   ├── web.md
│   └── cli.md
└── quickstart.md
```

`tasks.md` is produced by a later workflow, not this plan.

### Source Code

```text
bookmarker/model/             configuration and validation data
bookmarker/ports/             catalog and persistence boundaries
bookmarker/usecase/           configuration validation/activation and effective appearance
adapters/configyaml/         YAML loading and safe atomic activation
adapters/httpweb/            rendering, theme view context, protected form
adapters/jsonl/              versioned CLI configuration operations
adapters/                    thin Chroma catalog/asset adapter
cmd/bookmarker-{web,cli}/    composition roots
web/templates/               layout, pages, menu and fragments
web/static/                  application CSS and minimal JS
deploy/compose/              persistent configuration lifecycle
tests/{contract,integration,e2e,red-green}/
```

**Structure Decision**: Extend existing hexagonal library and both delivery adapters; wire a single bundled Chroma catalog in each process. Do not copy vdradmin-go's domain UI or implement config business rules in templates or JavaScript.

## Design and Delivery Sequence

1. Write failing unit and boundary tests first. Introduce a catalog port returning sorted ID/name options and exact membership; adapter constructs `chromaui.NewCatalog(nil)` once. Fail startup if the bundled catalog cannot be built. Both CLI and web compose the same validator. The loader accepts missing and unavailable persisted values so old installs and recovery work; only submissions enforce membership.
2. Add top-level YAML `theme` and redacted JSON field. Distinguish an omitted JSON property from an explicit empty/null value at the JSONL boundary: omitted means retain current selection, or System if never saved; explicit empty/null/unknown means validation error. An unavailable retained selection prevents all updates until replaced. Merge and validate the complete candidate before committing; keep secrets and existing config behavior intact.
3. Repair staged activation: detect short writes, sync/close staged file and open directory before atomic rename; preserve mode 0600. Rename is the commit point. Every error returned as a failed save must occur before rename and leave old bytes and active theme unchanged. After rename do not return a failed-save result for chmod/directory-sync failure: the staged file is already protected; report directory-sync failure as a structured durability warning and success with the new state, without claiming crash durability if sync failed. Test injected failures on both sides. Serialize each process's read/merge/validate/commit transaction; concurrent independent writers require a shared cross-process lock on a stable sibling lock file (not the renamed config inode), released on all paths, with bounded/error handling. Readers see old or new complete file. Do not log YAML, credentials or raw sensitive inputs.
4. On every full render, obtain the saved selection from the configuration service through a non-secret appearance projection; use `system` on missing, unavailable, or read failure, and log read failure without exposing configuration. The request-wide shell receives only the validated effective theme, not the entire configuration. Render System without a theme link or explicit `data-theme`; explicit IDs come only from the catalog. Keep native controls and HTML escaping. Load shared CSS, app layout CSS, selected theme CSS and compatibility CSS in the documented order; use Chroma components while preserving Bookmarker branding, roles, links, forms and fragments.
5. Populate the configuration dropdown from the same catalog; show a selected, clearly unavailable sentinel for an obsolete ID and require a valid replacement. Failed POST renders candidate form data and a useful error while the shell still uses the previous persisted appearance. Handle htmx 422 responses explicitly so errors are displayed rather than silently ignored. Successful normal POST redirects and successful htmx uses the existing `HX-Redirect`, forcing a full page load so head, shell, menu and content all immediately reflect the saved selection; retain database restart notice.
6. Make Compose store the complete config on an owner-only persistent named volume. Before upgrading an existing deployment, stop configuration changes and securely export `/run/bookmarker/config.yaml` from the old container before restarting or recreating it; keep an owner-only backup for rollback. Offer a one-time `BOOKMARKER_CONFIG_IMPORT_PATH` pointing to the export mounted read-only inside the new container. When the volume is empty, explicitly requested import must load and validate the old configuration, refresh only administrator username/verifier from validated deployment environment, and atomically initialize the volume with mode 0600; absent, unreadable or invalid import files must stop startup without initializing from defaults. If import is explicitly requested when the volume already contains configuration, stop with an actionable error and never overwrite it; remove the import setting and mount after successful migration before restarting. Without an import request, initialize an empty volume from the template. On subsequent starts preserve editable values, including theme, and atomically refresh only validated deployment-owned credentials. Never interpolate arbitrary secrets into shell commands or logs. Document volume ownership, export/import, secure backup, rollback and rotation; settings already lost before export cannot be recovered. Test migration, failure and retry cases, container restart and recreation, credential rotation and preservation of other settings.
7. Test and document all catalog themes, System, invalid/obsolete IDs, old JSON clients, injected failures, non-HTMX and HTMX saves, authorization, CSRF, existing journeys, responsive and keyboard use, asset/font failure, restart and DB-change notice. For restart tests, first test-drive an isolated Playwright server fixture with its own configuration and database, controllable stop/start and readiness checks; preserve its saved configuration across restarts without invoking the existing per-test reset. For every catalog theme and System, verify keyboard reachability, visible focus, labels and operation of all primary navigation items and Configuration controls. Update release/version and migration documentation; conduct participant studies for SC-001 and SC-006 rather than pretending automated tests prove them.

## Complexity Tracking

No constitution violation or new Repository/UoW is planned. The catalog port is necessary to validate the same bundled source in web and CLI without coupling domain logic to Chroma; a hard-coded list would drift. A shared cross-process config lock is needed because both CLI and server can write the same file; an in-process mutex cannot prevent lost updates across processes.
