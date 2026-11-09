# Bug Fix: Duplicate Configuration dropdowns

- **Slug**: duplicate-configuration-dropdowns
- **Fixed**: 2026-11-02
- **Assessment**: ./assessment.md
- **Status**: applied

## Summary

Configuration dropdown enhancement now commits only when the retained native select is actually removed from layout, painting, and pointer hit testing and the replacement is visible. Otherwise the original, labeled native select remains usable. HTMX swaps restore native controls before swapping and defer enhancement until settling is complete.

## Changes

| File | Change | Notes |
|------|--------|-------|
| `web/static/js/themed-select.js` | modified | Verify computed hiding and replacement visibility; roll back unsafe enhancement; preserve native classes, labels, attributes, and focus; recheck style changes and coordinate HTMX settling, including overlapping swaps. |
| `tests/e2e/themed-dropdown-options.spec.ts` | modified | Add missing/ineffective-style, whole-widget, theme, lifecycle, keyboard, validation, event, fallback, and HTMX regressions; ensure the no-JavaScript test includes the enhancement script. |
| `tests/e2e/themed-dropdown-desktop.spec.ts` | modified | Check the actual authenticated Configuration page, Spaceship red dark widget geometry and painting, and the exact missing-rule response across real 422 replacements. |
| `.specify/bugs/duplicate-configuration-dropdowns/fix.md` | added | Record the fix and verification. |

No stylesheet, template, deployment configuration, dependencies, or assessment file was changed.

## Tests Added or Updated

- `themed-dropdown-options.spec.ts` — **missing native-hiding rule falls back to usable native dropdowns**: removes only the assessed rule from the delivered stylesheet and checks all three labels, focus, keyboard selection, event counts, FormData, required validation, reset, and repeated 422 swaps.
- `themed-dropdown-options.spec.ts` — **fallback when ...**: covers an unavailable stylesheet, native layout/opacity/pointer/display overrides, and an invisible replacement.
- `themed-dropdown-options.spec.ts` — **enhanced controls occupy one row across System and every bundled theme** and **Spaceship red dark has no native paint anywhere in the whole form**: check all three controls across System light/dark and 37 bundled themes, plus a whole-form pixel comparison for the reported theme.
- `themed-dropdown-options.spec.ts` — **stylesheet loss and restoration preserve focus and never leave duplicate controls**: checks removal/reloading and a layout-changing CSSOM rule deletion.
- `themed-dropdown-options.spec.ts` — **enhancement preserves native form behavior and repeated HTMX recovery**: checks duplicate initialization, single input/change dispatch, values, reset, validation focus, and native-after-swap/enhanced-after-settle states across two intercepted fixture 422 responses.
- `themed-dropdown-options.spec.ts` — **overlapping HTMX swaps stay native until both have settled**, **a rejected HTMX response does not tear down the existing controls**, and **forced-colors teardown restores original labels, attributes, and focus**: cover simulated overlapping lifecycle events, an actual intercepted 403, and capability transitions.
- `themed-dropdown-options.spec.ts` — **no-JavaScript form submits native values**: now loads the enhancement script in the document while JavaScript is disabled. Existing touch/coarse-pointer and keyboard tests were also exercised.
- `themed-dropdown-desktop.spec.ts` — **desktop dropdowns preserve keyboard, native values and HTMX recovery** and **real configuration falls back when the native-hiding rule is missing**: verify the authenticated local application, all three whole widgets, the reported theme, form values, and 422 recovery. The latter strips only the native-hiding rule from the actual local server response.

## Local Verification

Playwright and Go commands used `TMPDIR="$PWD/test-results"`, `GOCACHE="$PWD/test-results/bug-fix-go-cache"`, `GOPROXY=off`, and `GOSUMDB=off`. The authenticated tests used an isolated fixture and the already-cached PostgreSQL image; browser requests to external HTTPS origins were aborted. No packages were installed.

- Before the fix, the exact missing-rule regression failed in both Chromium and Firefox: three enhanced wrappers remained instead of native fallback.
- `./node_modules/.bin/playwright test --config tests/e2e/themed-dropdown-options.config.ts --project=chromium --project=firefox --grep-invert 'opened DOM pickers use base-select|disabled selected and keyboard-active options remain legible|opened picker border contrasts'` → **54 passed, 4 skipped**. The skips are Firefox-specific unsupported or Chromium-only picker/forced-colors cases. Three unrelated long-running native-picker visual-contrast tests were excluded by the filter.
- `./node_modules/.bin/playwright test --config tests/e2e/themed-dropdown-desktop.config.ts --project=chromium --project=firefox` → **6 passed**, including the exact missing-rule case against the authenticated application.
- `./node_modules/.bin/playwright test --config tests/e2e/themed-dropdown-real.config.ts` → **2 passed**, including real normal submission without JavaScript.
- `go test -tags=contract ./tests/contract/... -run 'TestHTTPThemeSelectionAndInvalidSubmissions|TestHTTPConfigurationRendersRedactedValuesAndPreservesFieldValidation' -count=1` → **passed**.
- `node --check web/static/js/themed-select.js` and `./node_modules/.bin/tsc --noEmit --pretty false` → **passed**.
- `./node_modules/.bin/playwright test --config tests/e2e/themed-dropdown-options.config.ts --project=webkit --grep 'missing native-hiding rule falls back'` → **not run successfully**: WebKit could not launch because host libraries `libicu74` and `libflite1` are missing.
- Manual checks: inspected the local HTMX swap/settle transitions and the Firefox screenshot comparison. The latter was stabilized by keeping whole widgets in a fixed viewport because the theme's viewport-fixed gradient changed when scrolling. No reporter deployment or live screen reader was available.

## Deviations from Assessment

- The reporter's browser, build, and deployed CSS/JavaScript responses were not available, so the requested deployment comparison and cache-disabled reload could not be performed. No asset mismatch was established, and no caching or asset-versioning change was made.
- An additional instance of the same suppression failure was found locally: HTMX can restore server attributes after `htmx:afterSwap`, removing the JavaScript-added native-hiding class. The lifecycle now tears down before the swap and waits for `htmx:afterSettle`; this remains within the assessed JavaScript and test files.
- The overlapping-swap regression simulates the HTMX event sequence; the real-server tests exercise repeated 422 responses, not concurrent HTTP responses.
- Rechecks cover observed stylesheet DOM/load changes, layout changes, root theme-attribute changes, device/forced-colors media changes, resize, and HTMX events. A direct CSSOM-only property change that does not alter layout and produces none of those events is not continuously polled; it is rechecked on the next applicable event.

## Follow-ups

- Run `/speckit.bug.test slug=duplicate-configuration-dropdowns`.
- Capture the reporter's deployed CSS and JavaScript responses, browser/version, and a cache-disabled reload before attributing the original screenshot to a particular deployment cause.
- Run the browser coverage in a WebKit-enabled environment and, if available, verify with an actual screen reader.
