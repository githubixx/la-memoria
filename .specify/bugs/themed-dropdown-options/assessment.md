# Bug Assessment: Theme-aware dropdown options

- **Slug**: themed-dropdown-options
- **Created**: 2026-10-30
- **Source**: pasted text and user-supplied local screenshot
- **Verdict**: valid
- **Severity**: low

## Report (verbatim or summarized)

> In vdradmin-go the drop-downs have a way better styling as they also use the theme and not just the default grey one. We need this here too.

The supplied screenshot shows an open, dark purple theme dropdown with a bright pink selected row and a styled closed trigger. It is a visual reference, not proof of how la-memoria renders on a particular browser or operating system.

## Symptom

The current application has theme-aware CSS for the **closed** native select, but does not enable Chroma's enhanced option list. Its opened native popup is controlled to varying degrees by the browser and operating system, so its appearance can remain generic rather than match the screenshot. The requested consistent themed popup is therefore missing; the report should not be read as saying the closed control is always grey.

Severity is low: this is an appearance and consistency gap in the configuration interface, with no demonstrated loss of data or basic native-select functionality. Accessibility or readability failures on particular platforms would warrant reassessment.

## Reproduction

1. Sign in and open Configuration. The page contains Theme, Default view, and Database TLS mode selects.
2. Select a non-system theme such as Cartoon 1, save the configuration, and reopen Configuration. Open each select and compare its popup with the supplied reference.
3. Observe that the closed control uses theme styling, but the popup's appearance is browser/OS-dependent; native options may not reproduce the reference's themed surface and highlight.

[NEEDS CLARIFICATION: Which browser, version, operating system, and theme produced the reported grey popup? The screenshot's application and capture environment were not independently established.]

### Verification performed

An isolated temporary PostgreSQL fixture and headless Chromium and Firefox were used against the actual application; no existing browser session, tracked fixture, or shared test-output directory was used. External resources were blocked or replaced with empty stylesheets. The final probe loaded System (light preference), Cartoon 1, Light, and Dark by inserting the application's served theme CSS, rather than persisting a theme change. In both engines it found all three visible native selects and zero `.custom-select-container` widgets. Cartoon 1 produced a gradient closed control, light foreground, and `color-scheme: dark`; the selected DOM option's computed background was transparent. System/Light produced a white closed control and `color-scheme: light`. Keyboard ArrowDown changed Default view from `list` to `add`, and `FormData` contained the three native names and values. With JavaScript disabled, all three selects remained present and the Theme select was visible.

These are DOM/computed-style and limited interaction observations, **not** a visual test of the browser/OS-owned popup, a save/HTMX test, a contrast audit, or an assistive-technology test. Headless results do not establish the reported grey popup on a user's platform. The fixture and its build cache were removed after the probe.

## Suspected Code Paths

- `web/templates/pages/configuration.html:5-26` — the complete application-template inventory is three single selects: `theme`, `default_view`, and `database_tls_mode`. Theme is required and can contain a selected, disabled unavailable-theme option; the form submits normally or via `hx-post`.
- `web/templates/layouts/base.html:8-14` — loads Chroma base CSS, app CSS, the chosen theme, compatibility CSS, HTMX and application scripts, but not `/ui/js/select.js`.
- `go.mod:8` and `adapters/chromatheme/catalog.go:12-16,31-33` — pin Chroma UI v0.2.0 and serve its bundled catalog/assets; `adapters/httpweb/router.go:80-81` mounts its handler under `/ui/`.
- Pinned Chroma v0.2.0 `assets/css/base.css:47-52,751-770,1308-1422` — styles closed form selects and declares color schemes, plus styles for optional custom triggers, option lists, and selected states. Its explicit native option rule applies to multiple selects, not these single selects. Browser/OS popup rendering is not generally controllable to the same degree as DOM elements.
- Pinned Chroma v0.2.0 `assets/js/select.js:8,24-27,31-44,75-100,125-176` — optional enhancement creates a combobox/listbox around eligible native single selects, retains the native element for forms, and initializes on DOM ready and `htmx:load`. Its documented API is `window.ChromaUI.initSelects(root)`; native values changed programmatically need a `change` event for synchronization.
- `web/static/css/chroma-compat.css:8-13,25-32,76-81,90-95` — application layout, existing warning that theme surface/accent tokens are not inherently accessible popup pairs, forced-colors treatment for the menu, and focus styling that currently targets native selects rather than custom triggers.
- `adapters/httpweb/configuration_handlers.go:94-150` and `web/static/js/configuration.js:1-15` — configuration validates the submitted theme, returns a 422 form on error, swaps that form for HTMX, and redirects on success. A replacement widget must survive these paths.

### Reference and pinned API comparison

The local vdradmin-go checkout at commit `3a766b29074c467ff1d84929ea61cb6857cb5529` also pins Chroma v0.2.0. Its `web/templates/configurations.html:8-15` loads the Chroma base, its own theme compatibility CSS, and `/ui/js/select.js`; other reference templates do likewise. Chroma's optional enhancement is therefore a concrete explanation for its themed **DOM** popup, distinct from merely setting CSS on a native select. The reference was inspected, not run; its screenshot was not independently attributed to that checkout. No reference code needs to be copied.

Chroma documents label association, arrow/Home/End and Enter/Space handling, Escape/Tab closing, disabled-option skipping, form reset, validation focus, and one `input` plus one `change` on selection. Its implementation hides the original select with `display: none` and uses an ARIA combobox/listbox. Inspection of v0.2.0's key handler shows no printable-character typeahead; the supplied CSS has no explicit forced-colors treatment for the custom widget. These are integration risks, not proof that the existing native control fails. Disabled/hidden/multiple/size selects are skipped at initialization; dynamic state changes, accessible error announcements, and assistive-technology behavior must be verified rather than assumed.

## Root Cause Hypothesis

**Confidence: high** for the architectural gap: la-memoria loads Chroma's visual assets but not its optional select enhancement, leaving all three controls native. Their closed appearance is already themed; an OS/browser-managed native popup may ignore or reinterpret option styles, gradients, and selected-row colors. Exact reported popup colors and behavior remain unverified without the reporter's environment.

## Proposed Remediation

**Preferred**: Progressively enhance all three eligible configuration selects using the **existing pinned Chroma API/assets**, not copied reference code, provided the accessibility and form acceptance tests below pass. Keep the original named selects as the source of truth and retain native behavior when JavaScript or enhancement is unavailable. Use Chroma's documented initial and HTMX lifecycle, avoiding duplicate initialization; verify whether the bundled `htmx:load` listener is sufficient before adding any application-specific initialization. Add narrowly scoped app compatibility styles for full-width layout, clipping/stacking, focus, and theme-aware, opaque, legible list surfaces and selected/active/disabled states. Test System light and dark as well as bundled themes: gradients and translucent tokens do not by themselves guarantee a readable foreground/background pair.

Do **not** treat adding the script alone as an accessibility fix. Before enabling it, close or work around verified gaps such as typeahead, forced-colors behavior, validation/error communication, and dynamic disabled state, preferably in Chroma upstream or via an appropriately versioned dependency update. If those requirements cannot be met, retain the native control as the accessible fallback rather than ship a visually improved but less operable replacement. Preserve the existing Chroma GPL-3.0-or-later attribution and licensing; no new copying from vdradmin-go is proposed.

**Alternative**: Keep native selects and improve supported `color-scheme` and option foreground/background styling. This preserves mature platform accessibility and no-JS behavior, but cannot promise the screenshot's popup appearance across browsers, OSes, mobile pickers, or forced-colors modes. A different accessible select component is another option only if its licensing, maintenance, integration, and native-equivalent behavior are justified.

**Files likely to change**:
- `web/templates/layouts/base.html` — load the existing optional asset if its behavior is accepted.
- `web/static/css/chroma-compat.css` — scoped layout, accessible theme/color and forced-colors adaptations.
- `web/static/js/configuration.js` or a dedicated integration script — only if lifecycle or integration gaps cannot be handled by the documented Chroma API.
- `tests/e2e/` and `tests/contract/http_configuration_test.go` — targeted behavior, rendering, and server contract coverage; an upstream Chroma change or dependency update may also be necessary.

**Tests to add or update**:
- Inventory all three controls and assert one widget per eligible select on initial page load, after HTMX navigation and a 422 swap, and after repeated initialization; verify no orphaned overlays, duplicate listeners, or broken label focus.
- With keyboard alone, exercise opening, arrows, Home/End, Enter/Space, Escape, Tab, long-list scrolling, disabled-option skipping, and printable-character typeahead; ensure focus, active item, committed selection, and visible focus are distinguishable. Verify mouse/touch selection, outside click, narrow viewports, and no clipped or obscured dropdowns.
- Inspect accessible name, role, expanded state, active descendant, selected/disabled state, required/invalid state and error description; test actual screen readers with supported browser/OS combinations. Verify the unavailable disabled selected theme cannot be chosen or silently submitted, and that recovery offers an enabled replacement.
- Test native names/values, one `input` and `change` per committed change, reset, programmatic changes, disabled controls and disabled options, browser constraint validation and focus/error feedback. Cover valid and invalid normal POST and HTMX POST, CSRF, successful redirect, 422 re-render, and persisted values. Include the case where a selected disabled unavailable option is omitted from `FormData`; server validation must still reject it.
- Test JavaScript disabled and script-load failure: visible usable native selects, keyboard access, validation and normal form submission. Check initial disabled controls and disabled/enabled state changes after initialization.
- Across supported Chromium, Firefox, WebKit and representative mobile/OS environments, verify System light/dark and every bundled theme, including transparent/gradient surfaces: readable opaque popup backing; normal, hover, keyboard-active, selected and disabled states; adequate text contrast (normally at least 4.5:1) and focus/boundary contrast (normally at least 3:1 against adjacent colors). Test actual rendered/composited colors, not just CSS token values.
- Emulate `forced-colors: active` and test a real high-contrast environment: system colors and a visible focus/selection indicator must remain usable; use a native fallback if custom rendering cannot meet this. Visual screenshot tests of a custom DOM popup are useful, but do not claim to verify an OS-rendered native popup.

## Risks & Considerations

- A custom ARIA control trades native browser/OS behavior for predictable styling; it can regress typeahead, screen-reader announcements, touch/mobile interaction, and high-contrast accessibility. Automated accessibility checks alone are insufficient.
- Chroma's selected and active visual treatment, theme-specific overrides, gradients, and disabled opacity need contrast validation on actual composited backgrounds; do not assume all token combinations satisfy contrast.
- The required theme select may contain a selected disabled option; native form validation and successful-control serialization are not interchangeable. Preserve server-side validation and the error description during 422 swaps.
- HTMX replacement and dynamic control changes can leave stale widgets or handlers; native and enhanced states must remain synchronized without duplicate events or submissions.
- The local reference is evidence for an integration pattern, not a guarantee of accessibility, complete browser parity, or a request to copy its code.

## Open Questions

- [NEEDS CLARIFICATION: Which browser/version, operating system, theme, and popup appearance should be used to reproduce the reported grey dropdown?]
- [NEEDS CLARIFICATION: Is consistent screenshot-like popup styling required on mobile and across all supported platforms, or is a native fallback acceptable where accessibility or browser support requires it?]
- Which browser/assistive-technology combinations constitute the project's acceptance matrix?
