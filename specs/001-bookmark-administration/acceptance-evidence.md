# Acceptance Evidence

**Feature**: `001-bookmark-administration`  
**Recorded**: 2026-09-02

This record distinguishes automated verification from success criteria that
require a representative participant study.

| Criterion | Evidence | Status |
| --- | --- | --- |
| SC-001: 90% of first-time visitors find a known bookmark within 60 seconds | The supported-browser US1 suite verifies browse, exact tag filtering, all-word description search, direct URLs, htmx navigation, and 50-page pagination. No first-time participant timing study was conducted. | Not measured |
| SC-002: 90% of authenticated users add a reviewed capture and tags within 2 minutes | The supported-browser US2 suite verifies sign-in, capture preview, tag editing, and saving a bookmark. No first-attempt participant timing study was conducted. | Not measured |
| SC-003: 95% of reachable URLs produce preview/failure within 15 seconds | The managed local capture fixture produces a preview in the supported-browser US2 suite, and the unreachable fixture produces an explicit retry/continue state. Each test has a 20-second ceiling; successful capture completed in about 2.8 seconds in the recorded Chromium run. | Automated behavior verified; population rate not measured |
| SC-004: 95% of browse/filter/search actions complete within 2 seconds at 100,000 bookmarks | `TestPostgreSQLBrowseAndSearch100KBookmarksWithinTwoSeconds` passed against PostgreSQL 18.6. The test enforces a 2-second bound for browse and capped search; query-plan coverage requires index use. | Automated threshold verified |
| SC-005: all unauthenticated create/edit/delete/configure attempts are denied | Security E2E coverage denies unauthenticated bookmark, delete, capture, and configuration writes; cross-origin and stale-CSRF/session cases are rejected. HTTP/CLI contract and PostgreSQL integration suites pass. | Automated coverage verified |
| SC-006: 50+ page navigation reaches required pages within two selections | The US1 E2E scenario reaches pages 50 and 1 through visible pagination controls, with a 50-page seeded collection. | Automated behavior verified |
| SC-007: edits persist and confirmed deletes remove visible bookmark/screenshot state | The US3 E2E scenario verifies updated list state and subsequent search absence after confirmed deletion; contract and integration suites verify compensation paths. | Automated behavior verified |
| SC-008: 90% of administrators rate feedback at least 4/5 | Configuration validation and bookmark failure/continuation feedback are exercised by the US2 and US4 scenarios. No administrator feedback survey was conducted. | Not measured |

The supported browser matrix passed `78/78` scenarios in Chromium, Firefox,
and mobile Chromium. The matrix uses an isolated PostgreSQL 18.6 database,
fresh configuration and screenshot staging for every test, and a local capture
fixture. WebKit is intentionally excluded from the supported matrix.

## Scope Boundary

Representative timed first-time-user studies and administrator feedback
surveys are outside this feature's delivery scope. SC-001, SC-002, SC-003's
population-rate component, and SC-008 therefore remain unmeasured by design;
this record is limited to the available automated acceptance evidence.
