# Bug Assessment: Duplicate Configuration dropdowns

- **Slug**: duplicate-configuration-dropdowns
- **Created**: 2026-11-02
- **Source**: pasted text and user-supplied screenshot
- **Verdict**: likely valid, needs reproduction
- **Severity**: low

## Report (verbatim or summarized)

> There is an issue in Configuration with dropdowns. They appear twice. See screenshot.

The screenshot shows Theme set to "Spaceship (red dark)" and Default view set to "List". Each field has an upper native-looking select and a lower custom trigger with a filled triangle. The screenshot does not show Database TLS mode.

The user confirmed the slug interactively. No URL was supplied or fetched.

## Symptom

Configuration appears to render both the native select and its enhanced dropdown trigger, rather than one visible interactive control per field. The screenshot is credible evidence of the UI defect, but the exact failure was not reproduced with the complete current local assets.

Severity is low: duplicate controls cause visual clutter and confusing interaction in the administrative Configuration form. No data loss, persistence failure, or wider application failure has been demonstrated.

## Reproduction

### Reporter scenario

1. Open Configuration in an authenticated session.
2. Use the "Spaceship (red dark)" theme, as shown in the screenshot.
3. Inspect the closed Theme and Default view controls: the screenshot shows two stacked controls per label.

[NEEDS CLARIFICATION: Browser/version, operating system, application URL/build, and whether a full reload with browser cache disabled still shows duplicates.]

### Verification performed

An isolated Playwright probe used an intercepted HTTP origin, the actual installed Chroma v0.2.0 assets, the current application stylesheets, and the current `themed-select.js`. Representative Configuration markup included all three named selects and their labels. External requests were aborted. No application server, database, saved configuration, source files, or shared browser session was changed.

- **Current complete assets, Chromium and Firefox**: three custom triggers were created. Every native select had computed `position: absolute`, `opacity: 0`, and `clip-path: inset(50%)`; it occupied no additional layout row. The accessible combobox count was three, not six. Chromium measured the clipped native boxes at 24 x 24 pixels and Firefox at 1 x 1 pixels; neither measurement implies visible painting because opacity and clipping were applied. The screenshot's stacked duplication was not reproduced.
- **Controlled missing-style simulation, Chromium and Firefox**: the routed compatibility stylesheet was altered only in memory to remove `.form-grid .themed-select-native`. All other assets remained current. Every native select then had `position: static` and `opacity: 1`, with a separate custom trigger below it. Chromium measured both rows at 44 pixels high; Firefox measured native rows at 42 pixels and trigger rows at 44 pixels. This reproduces the screenshot's two-row structure without creating duplicate widgets.
- **WebKit**: launch failed because the host is missing browser system dependencies, including `libicu74` and `libflite1`. No packages were installed on the shared host. WebKit behavior remains unverified.

These are representative-markup DOM/computed-layout probes, not a reproduction against the reporter's deployed application, an authenticated end-to-end test, or a full visual/accessibility audit. The probes created no persistent scripts or screenshots.

## Suspected Code Paths

- `web/templates/pages/configuration.html:8` and `:18` — declare one native select each for Theme and Default view. Database TLS mode is the third select at line 26. The template does not duplicate these controls.
- `web/static/js/themed-select.js:8` — `enhance` creates a `.themed-select` wrapper and `.themed-select-trigger` button alongside the original select.
- `web/static/js/themed-select.js:48` — adds `.themed-select-native`, `tabindex="-1"`, and `aria-hidden="true"` to the retained native control. These attributes remove keyboard/accessibility exposure but do not themselves suppress visible rendering.
- `web/static/js/themed-select.js:213` — `refresh` guards initialization with an `enhanced` map, responds to HTMX swaps and device-media changes, and restores native state during teardown.
- `web/static/css/chroma-compat.css:35` — the dedicated rule hides and removes enhanced native selects from normal flow. Removing only this rule reproduces the stacked native/custom pair.
- `web/templates/layouts/base.html:11` and `:15` — load the compatibility stylesheet and enhancement script using separate, unversioned asset URLs.
- `cmd/bookmarker-web/main.go:133` — serves CSS and JavaScript from separate filesystem directories via `http.FileServer`. Inspection found no application-specific cache policy on these asset registrations; this does not establish that the reporter received stale assets.
- `tests/e2e/themed-dropdown-options.spec.ts:118` — checks native painting over the custom trigger for Spaceship 2, not the screenshot's Spaceship red dark theme. Its trigger-only screenshots would not necessarily detect a separate visible native row above the trigger.
- `tests/e2e/themed-dropdown-desktop.spec.ts:7` — exercises desktop enhancement and HTMX recovery against the application, but does not directly assert that the retained native selects cannot paint or consume extra layout rows.

## Root Cause Hypothesis

**Confidence: high for the duplicate-rendering mechanism; medium for its applicability to the reporter's environment; low for any specific deployment/cache explanation.**

The native select is intentionally retained as the form's source of truth, and JavaScript adds one custom trigger beside it. Hiding the native element depends on a separate CSS rule. If enhancement runs without that rule taking effect, the browser paints both controls in consecutive rows, exactly as the controlled probe demonstrated. JavaScript currently checks CSS feature support, not whether the required stylesheet or native-hiding rule actually loaded. A stale/partial asset deployment, cached older stylesheet, failed stylesheet load, or environment-specific cascade difference could explain the report, but none was observed directly. With all current local assets present, both tested engines hide the natives correctly. There is no evidence here of duplicate template markup or repeated enhancement of the same element.

## Proposed Remediation

**Preferred**: First capture the affected browser's response for `/assets/css/chroma-compat.css` and computed styles for `#theme.themed-select-native`, alongside the loaded enhancement script. Compare those responses with the intended deployment and retest after a cache-disabled reload. This distinguishes stale/mismatched assets from a genuine cascade or browser issue before changing working styles.

Make native hiding and progressive enhancement reliably succeed or fall back together. If the required hiding styles are unavailable or ineffective, retain one usable native select instead of exposing a second trigger while marking the native control inaccessible. Prefer a small change within the existing enhancement/teardown lifecycle, preserving the original native state and validation behavior. If an asset mismatch is confirmed, correct deployment consistency and use an appropriate asset revalidation/versioning strategy rather than adding an unrelated theme-specific override.

Do not delete the native controls: form submission, required validation, reset, change events, and no-JavaScript/device fallbacks depend on them.

**Files likely to change**, depending on confirmed cause:

- `web/static/js/themed-select.js` — coordinate successful enhancement, native suppression, and restoration.
- `web/static/css/chroma-compat.css` — only if the delivered rule has a confirmed cascade/coverage problem.
- `web/templates/layouts/base.html`, `cmd/bookmarker-web/main.go`, or deployment asset packaging — only if inspection establishes an asset consistency/caching defect.
- `tests/e2e/themed-dropdown-options.spec.ts` — add exact-theme, whole-widget geometry/painting and missing-style coverage.
- `tests/e2e/themed-dropdown-desktop.spec.ts` — verify the same invariant on the actual authenticated Configuration page, including HTMX replacement.

**Tests to add or update**:

1. With Spaceship red dark, check all three Configuration fields have exactly one visible, operable dropdown. Check the entire widget area, not only trigger screenshots or accessible role counts: `aria-hidden` can conceal an incorrectly painted native from role queries.
2. Test System light/dark and bundled themes in supported browser engines. Verify native elements neither paint nor consume separate layout rows after enhancement.
3. Simulate a missing native-hiding rule or unavailable compatibility stylesheet. Verify clean native fallback or otherwise atomic enhancement, with correct labels, keyboard focus, and no inaccessible visible duplicate.
4. Repeat initialization and HTMX 422 replacements; confirm no duplicate wrappers and one interactive control per field.
5. Preserve native FormData names/values, required validation/error focus, reset, single input/change event dispatch, and teardown when entering forced-colors or coarse-pointer mode.
6. With JavaScript disabled, confirm one visible native select per field.

## Risks & Considerations

- The working tree already contains extensive unrelated and related theme/UI changes. This assessment preserves all of them and does not attribute the screenshot to a particular commit.
- Changing how native selects are hidden can affect browser validation and focus handling; test these paths before selecting a hiding mechanism.
- A blanket `display: none` or theme-specific specificity increase is not justified by the current evidence and must not break fallback behavior.
- An accessible combobox count alone is insufficient: the current enhancement marks retained native elements `aria-hidden`, even when missing CSS leaves them visible.
- No API changes, schema migrations, dependency updates, or database changes are indicated.

## Open Questions

- [NEEDS CLARIFICATION: Which browser/version and operating system produced the screenshot?]
- [NEEDS CLARIFICATION: Which build and delivered CSS/JS assets are loaded, and does the issue persist after a cache-disabled full reload?]
- [NEEDS CLARIFICATION: Does Database TLS mode also appear twice, and does duplication occur outside Spaceship red dark?]
