# Bug Fix: Theme-aware dropdown options

- **Slug**: themed-dropdown-options
- **Fixed**: 2026-10-30
- **Assessment**: ./assessment.md
- **Status**: fixed for tested desktop browsers; additional accessibility and platform validation remains

## Desktop enhancement follow-up

The user chose a custom themed popup on desktop, with native controls on mobile and in high-contrast mode. The Configuration selects are now progressively enhanced only when the primary input supports hover and a fine pointer, forced colors are off, and the required CSS colors are supported. The original selects remain the submitted values and are restored if those capabilities change. Without JavaScript, the original controls remain available. Native constraint-validation errors are shown and associated with the visible control.

For Spaceship 2, the theme overrides the native select's clipping and draws its SVG border over the enhanced trigger. The enhanced-only native control is also transparent and ignores pointer events, preventing that artifact while retaining form and validation behavior. Following the user's reference, the visible trigger reuses Chroma's Spaceship 2 frame; the popup uses a black background and cyan selected/active options with dark text. Headless checks cover the frame, popup colors and native-control overlay in Chromium and Firefox.

The enhancement provides listbox keyboard navigation, typeahead, disabled-option handling, scrollable options, viewport-aware placement, outside-click and Tab dismissal, label and error associations, reset/change synchronization, and reinitialization after HTMX swaps. It does not import the optional Chroma JavaScript or copy its implementation. The previous native-picker styles remain as fallback.

Additional changes:

- `web/static/js/themed-select.js` and `web/templates/layouts/base.html`: progressive enhancement and script loading.
- `web/static/css/chroma-compat.css`: themed popup, trigger and option styling.
- `tests/e2e/themed-dropdown-desktop.spec.ts` and its dedicated config: isolated actual-server tests in Chromium and Firefox.
- `tests/e2e/themed-dropdown-options.spec.ts`: enhanced popup color, keyboard, disabled-state, reset, validation, geometry, and emulated touch and capability-change checks, alongside the existing native-picker tests.
- `tests/e2e/themed-dropdown-real.spec.ts`: actual enhanced picker interaction while retaining native form-value and no-JavaScript save checks.
- Both picker test configs now use unique system-temporary output directories.

Verification after the follow-up, all headless:

- `themed-dropdown-desktop.config.ts`: 4 passed in Chromium and Firefox, including actual HTMX 422 recovery and save and emulated forced-colors fallback.
- Enhanced and touch cases in `themed-dropdown-options.config.ts`: 8 passed in Chromium and Firefox. Computed opaque option foreground/background contrast was at least 4.5:1 for normal and active options in 37 bundled themes and both System appearances; this is not a pixel-based or comprehensive accessibility audit.
- A separate screenshot-pixel check of rendered normal and active option text passed a 4.5:1 threshold in Chromium and Firefox for System light/dark and the available Dark, CRT and Aurora Glass 1 themes. This samples glyph pixels, not every rendered pixel or state.
- The combined native and enhanced picker suite passed 23 tests, with 7 Firefox native-picker cases skipped where the relevant browser feature was unavailable. The final focused enhanced and touch run passed 10 tests.
- `themed-dropdown-real.config.ts`: 2 passed in Chromium, including a real JavaScript-free form submission.
- `go test -tags=contract ./tests/contract/... -run TestHTTPTheme -count=1`, `npx tsc --noEmit`, `node --check web/static/js/themed-select.js`, and `git diff --check`: passed.

WebKit remains unverified because its host libraries were unavailable in the earlier attempt. Physical touch devices, actual OS high-contrast environments and screen readers have not been tested; emulation and automated checks do not replace those checks. Programmatic changes to the native select should dispatch a `change` event so the visible value is synchronized.

## Previous native-only implementation

## Summary

Added theme-accented, opaque option colors and progressively styled native pickers in browsers supporting `appearance: base-select`. Native selects remain the form controls and the fallback. The fix is partial because a consistently themed opened popup and accessibility have not been established on all browser/OS/assistive-technology combinations; browsers without customizable selects retain their platform picker.

## Changes

| File | Change | Notes |
|------|--------|-------|
| `web/static/css/chroma-compat.css` | modified | Theme-tinted paired foreground/background colors, native picker, selected/hover/focus/disabled states, scrolling, and native forced-colors fallback. This file was already untracked work before the fix; existing content was retained. |
| `tests/e2e/themed-dropdown-options.spec.ts` | added | Isolated headless picker, composited contrast, form, keyboard, touch emulation and fallback checks against actual local CSS assets and representative markup. |
| `tests/e2e/themed-dropdown-options.config.ts` | added | Headless multi-browser configuration with a unique output directory. |
| `tests/e2e/themed-dropdown-real.spec.ts` | added | Actual isolated application, PostgreSQL, HTMX validation/recovery, persistence and no-JavaScript POST checks. |
| `tests/e2e/themed-dropdown-real.config.ts` | added | Headless Chromium configuration with a unique output directory. |

## Tests Added or Updated

- `themed-dropdown-options.spec.ts`: opened-option pixel contrast and geometry for all three selects, disabled and keyboard-active option contrast, picker boundary contrast, native semantics, reset, events, disabled values, dynamic state, keyboard, scroll, outside click, narrow touch emulation, System/theme changes, HTMX replacement and forced-colors emulation.
- `themed-dropdown-real.spec.ts`: real Configuration popup and save, unavailable disabled selected option and omitted value rejection through 422 swaps, replacement selection, persistence, and normal form submission with JavaScript disabled.
- Existing `tests/contract/http_configuration_test.go` theme tests were run without alteration, including invalid submissions and recovery.

## Local Verification

All browser runs were headless. The mock fixture serves actual pinned Chroma and application CSS locally and blocks external requests; it is not the real application. The real-application tests used a locally cached PostgreSQL image, a uniquely named container and a fresh temporary fixture, and blocked external resources. Playwright outputs used unique directories; existing shared outputs and tracked fixtures were not removed.

- `npx playwright test --config tests/e2e/themed-dropdown-options.config.ts --project chromium` → 10 passed. The subsequently added boundary test and strengthened dynamic-disabled test were run separately and passed.
- `npx playwright test --config tests/e2e/themed-dropdown-options.config.ts --project chromium --grep 'border contrasts'` → 1 passed.
- `npx playwright test --config tests/e2e/themed-dropdown-options.config.ts --project chromium --grep 'keyboard, dynamic'` → 1 passed after adding dynamically disabled selected-value checks.
- `npx playwright test --config tests/e2e/themed-dropdown-options.config.ts --project firefox` → 4 passed, 6 skipped in the run before the additional boundary test. Firefox used the native fallback; these checks did not visually inspect its OS popup.
- `npx playwright test --config tests/e2e/themed-dropdown-options.config.ts --project webkit` → 8 tests failed at browser launch, before test execution: host dependencies `libicu74` and `libflite1` were unavailable. No system libraries were installed.
- `GOPROXY=off GOSUMDB=off npx playwright test --config tests/e2e/themed-dropdown-real.config.ts` → 2 passed. An initial attempt to save TLS mode `require` returned 422 with a generic save error; its precise cause was not established. The final test keeps the fixture's connection in `disable` mode and passes.
- `GOPROXY=off GOSUMDB=off go test -tags=contract ./tests/contract/... -run TestHTTPTheme -count=1` → passed.
- `npx tsc --noEmit` → passed.

In the headless Chromium mock, screenshot pixels from the **opened** picker were compared with the same rendered scene with text hidden. Across 37 bundled themes plus System light and dark, 351 normal/selected/hover option-state samples across the three selects had a minimum sampled text contrast of 6.54:1. The selected disabled and keyboard-active theme-option samples had minima of 5.71:1 and 6.54:1 respectively. At one sampled point on the Default view picker boundary per theme/mode, minimum border/interior and border/exterior contrast was 15.07:1 and 10.78:1. These are measurements of the tested pixels and fixture, not guarantees for every pixel, state, background, browser or device; focus-indicator contrast was not comprehensively measured.

The real application’s opened Cartoon 1 picker was captured and visually inspected at `test-results/themed-dropdown-real-b0046cb2-0e13-4435-8cbd-94243ee23321/themed-dropdown-real-real--3830f-er-validation-swap-and-save-chromium/cartoon-open-picker.png`. This ignored output belongs to the unique test run and is not a permanent fixture.

## Deviations from Assessment

- Used the assessment's native-select alternative rather than loading Chroma's optional JavaScript enhancement. Inspection confirmed that the pinned enhancement does not provide typeahead or teardown, can become stale when options change, and hides the native select while suppressing native validation feedback. Enabling it without resolving those gaps would risk regressions. No Chroma or reference-application code was copied, and no dependency was added.
- Where supported, customizable native selects provide a DOM-rendered themed popup without replacing native form controls. Elsewhere, CSS supplies option colors where honored, but the platform controls the popup and may ignore them. No claim is made that Firefox, WebKit or every mobile picker matches the screenshot.
- The new tests use additional dedicated files under the assessment's `tests/e2e/` scope. No server or template changes were necessary.

## Follow-ups

- Run `/speckit.bug.test slug=themed-dropdown-options`; validate WebKit with its host dependencies and visually inspect Firefox and other supported browser/OS-native popups.
- Test real screen readers with supported browser/OS pairs, real mobile/touch devices, actual high-contrast environments and focus/boundary contrast on representative surrounding content. The emulated forced-colors test verified focus and keyboard operation, not a real OS picker or its visual contrast.
- If identical screenshot-like rendering is required on platforms without customizable native selects, resolve the Chroma enhancement's accessibility and dynamic-state gaps upstream and validate its behavior before adopting it.
