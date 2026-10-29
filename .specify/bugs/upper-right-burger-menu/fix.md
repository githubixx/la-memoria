# Bug Fix: Upper-right burger menu on every view

- **Slug**: upper-right-burger-menu
- **Fixed**: 2026-10-29
- **Assessment**: ./assessment.md
- **Status**: applied

## Summary

Replaced the desktop link bar and mobile text toggle with an accessible upper-right burger dropdown at every viewport width. Menu behavior now loads through the shared layout; links remain usable without JavaScript, and Add/Edit form behavior initializes once for each form after direct loads or htmx swaps.

## Changes

| File | Change | Notes |
|------|--------|-------|
| `web/templates/fragments/menu.html` | modified | Three-line icon, accessible Menu name, native tooltip; existing destinations, authentication branches, and CSRF field preserved. |
| `web/static/css/bookmarker.css` | modified | All-width, right-aligned dropdown; 44px control, viewport-constrained panel, wrapping labels, scroll handling, and stacking. Collapsed styling applies only after successful enhancement. |
| `web/static/js/bookmark-form.js` | modified | Outside activation, focus departure, navigation dismissal, synchronized disclosure state, and idempotent form initialization after htmx swaps. |
| `web/templates/layouts/base.html` | modified | Load the shared script on every full-page view. |
| `web/templates/pages/add.html` | modified | Remove duplicate page-local script load. |
| `web/templates/pages/edit.html` | modified | Remove duplicate page-local script load. |
| `tests/e2e/accessibility-responsive.spec.ts` | updated tests | Burger appearance, placement, disclosure/dismissal, viewport containment, authentication, direct/htmx navigation, form initialization, and no-JavaScript links. |

No backend, schema, dependency, or theme changes were introduced. The assessment was not edited.

## Tests Added or Updated

- `tests/e2e/accessibility-responsive.spec.ts` - parameterized burger checks at 320px, 390px, and 1280px across fresh List, Search, and sign-in views; assert icon markup, accessible name, touch target size, closed/open state, destinations, and repeated toggling.
- The same viewport checks capture screenshots, exercise long unbroken labels, dismiss via outside activation, and verify scrolling to the last item in a 240px-high viewport.
- Keyboard-only navigation now verifies Escape closes the dropdown and returns focus to its control.
- Navigation checks distinguish htmx swaps from ordinary full-page navigation, verify automatic dismissal, and exercise signed-out Add/Configuration redirects.
- Administrative checks cover fresh Add, Edit, and Configuration loads; repeated htmx Add/Edit transitions; tag handlers without duplication; capture request initialization; successful sign-out with the existing CSRF form; and protected access after sign-out.
- The no-JavaScript case now follows actual List/Search links rather than testing only a direct URL.

## Local Verification

- Red baseline: `npm run test:e2e -- tests/e2e/accessibility-responsive.spec.ts --project=chromium --grep 'burger dropdown.*1280px'` failed before implementation because the desktop Menu control was not visible.
- The same desktop check passed after implementation.
- `npm run test:e2e -- tests/e2e/accessibility-responsive.spec.ts` - **27 passed**, across Chromium, Firefox, and mobile Chromium, in 28.9s after correcting two test assumptions: the outside-click target must not be covered by the dropdown, and removing `hx-get` does not unbind an already initialized htmx link.
- `npm run test:e2e -- tests/e2e/us2-add-bookmark.spec.ts tests/e2e/us3-maintain-bookmarks.spec.ts` - **20 passed, 1 failed**. The failing existing test is the mobile successful-capture/save journey, where capture-preview overflow causes pointer interception at the Save button. All desktop journeys and the other mobile journeys passed.
- `npm run test:e2e -- tests/e2e/us2-add-bookmark.spec.ts --project=mobile --grep 'saving after a successful capture'` - the same failure reproduced independently.
- An isolated Playwright comparison served `HEAD` versions of the pre-fix CSS/JavaScript via request interception and restored the old text toggle in that browser context, without modifying the checkout. The save click was blocked with both pre-fix and fixed assets; both rendered a 1280px preview and reported a 1304px document/layout viewport. This supports classifying the capture issue as unrelated to the burger fix, not a fully green Add/Edit suite.
- `git diff --check`, JavaScript syntax checking with `node --check web/static/js/bookmark-form.js`, and trailing-newline checks passed for the seven touched source/test files. Editor diagnostics reported no errors in the CSS, JavaScript, or updated TypeScript test file.
- Manual visual checks: inspected Playwright screenshots at 320px and 1280px; the burger is upper-right and the open panel is contained, legible, and consistent with the existing light theme.
- Preview: `http://127.0.0.1:18080/bookmarks`, using isolated PostgreSQL and capture fixtures from the existing E2E setup. The preview process remains running. The setup regenerates the config template removed by its normal test teardown; no template deletion is intended as part of this fix.
- Broader backend/integration suites were not run because no backend contracts changed.

## Deviations from Assessment

None. Changes remain within the listed implementation/test files, plus this required report. The existing theme was retained; Chroma UI was not introduced. Browser runs confirmed the menu defect, resolving the assessment's outstanding reproduction question.

## Follow-ups

- Run `/speckit.bug.test slug=upper-right-burger-menu` for the formal verification report.
- Assess the separate mobile capture-preview overflow/save-click failure; it is deliberately not fixed or hidden by this navigation change.
- The existing E2E teardown deletes a tracked generated config template. Consider addressing that harness behavior separately so future test runs do not leave a deletion in the worktree.
