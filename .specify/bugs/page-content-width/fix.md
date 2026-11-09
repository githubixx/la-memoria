# Bug Fix: Page forms match bookmark content width

- **Slug**: page-content-width
- **Fixed**: 2026-10-30
- **Assessment**: ./assessment.md
- **Status**: applied

## Summary

Page forms now use the same available width as bookmark list cards without changing the centered main container or the nested menu form. Long unbroken form tags and page messages wrap within the available width.

## Changes

| File | Change | Notes |
|------|--------|-------|
| `web/static/css/chroma-compat.css` | modified | Scoped page-form width override and wrapping for tags, labels, and messages; retained grid safeguards and existing control sizing. |
| `tests/e2e/page-content-width.config.ts` | added | Headless browser projects with unique temporary output paths and no shared setup. |
| `tests/e2e/page-content-width.spec.ts` | added | Isolated real-server/database width, responsive, long-content, and interaction regression checks. |

The existing grid policy remains: text fields, selects, numbers and action buttons stretch with the form; checkboxes remain intrinsic. The menu's nested sign-out form is not targeted.

## Tests Added or Updated

- `tests/e2e/page-content-width.spec.ts` — compares actual list card and page form edges for Sign in, Search, Add, Edit, Configuration and Delete at 320, 390, 480, 481 and 1514 CSS pixels across system light/dark, explicit light/dark and Aurora. Checks search result and field widths, menu/logout bounds and keyboard Escape, direct navigation, HTMX navigation and search, authentication and sign-out, native required validation, Configuration's 422 swap, tag add/remove, long inputs and unbroken tags, create/edit/delete actions, and overflow in the tested stress cases.
- Uses a fresh temporary server, config, screenshot directory and PostgreSQL container with a unique name. External browser requests are blocked; the database image must already exist locally. The fixture cleans up only its own resources.

## Local Verification

- `npx playwright test --config tests/e2e/page-content-width.config.ts --project chromium --project firefox --project mobile --workers 3` → 3 passed, headless. Mobile uses Chromium with iPhone device emulation. Output directories are uniquely named under the system temporary directory.
- `npx playwright test --config tests/e2e/page-content-width.config.ts --project firefox --project webkit --project mobile` → Firefox and mobile passed; WebKit could not launch because the host is missing `libicu74` and `libflite1`.
- `go test ./bookmarker/... ./adapters/httpweb/...` → passed.
- `npx tsc --noEmit --pretty false` → passed.
- Manual checks: none; dimensions and interactions were checked in automated browsers.

## Deviations from Assessment

- Added wrapping for top-level page paragraphs as well as form tags and validation messages: the Delete confirmation includes the bookmark description outside the form and can otherwise overflow with unbroken text. This is the tightly coupled long-content case authorized by the request.
- WebKit verification remains blocked by missing host libraries. Browser zoom, physical devices, all bundled themes, capture retry/discard and preview lifecycles, and unavailable-theme feedback were not independently exercised in this run. The test did not make an external capture request; it submitted a bookmark with the existing continue-without-screenshot control instead.

## Follow-ups

- Install the missing WebKit host dependencies in an appropriate test environment and rerun the WebKit project.
- Run `/speckit.bug.test slug=page-content-width`, including the remaining capture, zoom and unavailable-theme cases where the environment permits.
