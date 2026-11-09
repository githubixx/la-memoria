# Bug Assessment: Page forms narrower than bookmark list

- **Slug**: page-content-width
- **Created**: 2026-10-30
- **Source**: pasted text and user-supplied local screenshot
- **Verdict**: valid
- **Severity**: low

## Report (verbatim or summarized)

> Currently, besides the list page (see screenshot), only a small portion of the page is used and the form fields are quite small. All pages should use the width as the bookmark list item shown in the screenshot.

The supplied screenshot shows a bookmark card approximately 912 px wide in a 1514 px-wide image, centered with the page heading. It is the reference for the **content width**, not a request to make the entire viewport full-width.

## Symptom

At desktop widths, page forms and their grid-stretched fields occupy substantially less horizontal space than bookmark cards, even though both sit in the same main content container. Page forms should make available the same content width as the list item; individual control widths and responsive behavior still need appropriate treatment.

## Reproduction

1. At a desktop viewport around 1280–1514 px, open the bookmark list and observe a list item.
2. Open Search or Sign in; with an authenticated session, open Add, Edit, Delete, or Configuration. Compare their page form widths with the list item. Search results also show the mismatch between the form and the results below it.
3. At a narrow viewport around 320–390 px, compare again: the available content width, rather than the desktop form cap, constrains both.

**Measured evidence:** Headless Chromium rendered the actual Go templates through the application renderer with representative fixture data and the local application and Chroma v0.2.0 stylesheets, without starting the application, touching its database, or using the shared E2E setup. The system light/dark color schemes, explicit `light`, `dark`, and screenshot-like `aurora-glass-1` themes all produced the same widths. Values are CSS pixels; the desktop-style browser context reserves a 15 px scrollbar gutter, including at the narrow viewport sizes.

| Viewport | Main outer width | List card | Page form / typical text, URL, number or select control |
|---|---:|---:|---:|
| 1514 or 1280 px | 960 px | 912 px | 512 px |
| 390 px | 375 px | 343 px | 343 px |
| 320 px | 305 px | 273 px | 273 px |

At desktop the form is 400 px narrower than the card (about 56% of the card width). Search, Add, Edit, Configuration, Sign in, and Delete all showed the 512 px form cap. Search's results card measured 912 px alongside its 512 px form. Add/Edit tag and submit buttons, Search's submit button, and Delete's button also stretched to the form width; the screenshot checkbox stayed 13 px. Configuration's numeric inputs and selects measured the full form width. The open menu measured 240 px and its nested logout form 222 px at the tested sizes.

In the ordinary fixture, no horizontal document overflow was measured at the tested viewport sizes. A separate 320 px Aurora test adding a 280-character unbroken tag using the real tag script produced 2256 px document scroll width; the tag could be removed, and the empty required URL and description failed native validity checks. This is a pre-existing, related responsive edge case, not evidence that the width cap caused it.

**Limitations:** These were representative template fixtures, not a running end-to-end authenticated application or physical-device emulation. No server submissions, HTMX navigation/error swaps, capture lifecycle, actual validation response, screenshots of all pages, Firefox/WebKit, zoom, or all bundled themes were exercised. The screenshot's exact viewport/zoom and the desired sizing of individual controls are not independently known.

## Suspected Code Paths

- `web/static/css/bookmarker.css:129` — `main#content` is centered, padded, and capped at `60rem` (960 px at the measured 16 px root size); this is the shared page container, not the narrow-form cause.
- `web/static/css/bookmarker.css:135` and `web/templates/fragments/bookmark_results.html:5` — list/grid and card occupy the main container's content area; 24 px padding on each side yields the 912 px desktop reference.
- `web/static/css/bookmarker.css:181` — the global `form` rule sets grid layout and `max-width: 32rem` (512 px); it also matches the nested logout form.
- `web/static/css/chroma-compat.css:8` — `.form-grid` uses a single `minmax(0, 1fr)` column, with control max-width safeguards; URL and number inputs have explicit `width: 100%` at lines 11–18. Grid stretching also affects buttons and other fields, so a wider form affects more than text fields.
- `web/templates/pages/search.html:3`, `add.html:4`, `edit.html:4`, `configuration.html:5`, `login.html:4`, and `delete.html:4` — all use `.form-grid` on their page forms. Login is a page form inside `main`, whereas logout is a different, nested form in the menu.
- `web/templates/fragments/menu.html:13` and `web/static/css/bookmarker.css:72` — logout sits inside a 15rem, viewport-bounded menu; avoid inadvertently applying page-form widths or control rules to it.
- `web/templates/fragments/capture.html:1`, `web/templates/pages/add.html:10`, and `web/static/js/bookmark-form.js:41` — dynamically rendered capture status, preview/retry controls, tags, and the screenshot checkbox need responsive treatment alongside the form; long tags can currently overflow.
- `web/templates/pages/configuration.html:3` and `web/static/js/configuration.js:1` — configuration validation messages and the special HTMX 422 swap path should remain usable and visible after layout changes.

## Root Cause Hypothesis

**Confidence: high.** The 32rem maximum on the global form selector caps page forms while the list card uses the 60rem main container's padded content width. Since the single grid column stretches its children, the cap also limits most fields and buttons. At small viewport widths the shared available width falls below 32rem, masking the difference. Increasing the main container cap alone would not resolve the form cap and would change the list reference.

## Proposed Remediation

**Preferred**: Keep the existing centered `main#content` max-width and its responsive padding. Scope a full-available-width form rule to the actual page forms in `main` (for example `main#content > form.form-grid` or an explicit page-form class), removing or overriding their 32rem cap so their outer edges align with the list card content area. Preserve grid/min-width safeguards, including after HTMX swaps. Do not blanket-widen every `form`: the menu logout form is a separate nested control.

Decide control sizing intentionally rather than forcing every descendant to `width: 100%`. Long text/URL fields and relevant selects should be able to use the available width; checkboxes must remain intrinsic, and buttons, short numeric fields and selects should have a deliberate, consistent sizing policy. The current grid stretches buttons and numbers, so changing only the form cap would also enlarge them. Preserve labels, focus, required and server validation messages, capture preview/status, tags and action visibility. Address or separately track the observed long-unbroken-tag overflow, without conflating it with the original width cause.

**Alternative**: Increase the global form cap to the main width. This is simpler but also changes unrelated nested forms and may produce oversized controls; a scoped page-form rule provides clearer ownership and safer menu behavior.

**Files likely to change**:
- `web/static/css/bookmarker.css` and/or `web/static/css/chroma-compat.css` for scoped layout and any intentional control sizing.
- `web/templates/pages/*.html` only if an explicit page-form class or targeted markup is needed; `web/templates/fragments/menu.html` should remain isolated.
- New or existing Playwright E2E tests, for example `tests/e2e/chroma-journeys.spec.ts` or a dedicated isolated width spec.

**Tests to add or update**:
- At desktop, assert page form edges/available width match a list card's content edges within a small pixel tolerance on Search, Add, Edit, Configuration, Sign in, and Delete; compare Search's form with its own results. Check both direct navigation and HTMX content swaps.
- At 320, 390, around the 480 px padding breakpoint, and a wide viewport, assert no unintended horizontal scrolling, clipping, or inaccessible focus; include actual mobile emulation, browser zoom, and long URLs, descriptions, tags, validation messages and capture-status/preview content.
- Exercise text/URL/number/select fields, theme selection, checkbox, tag add/remove, capture retry/discard, submit/delete actions and error feedback. Check native invalid fields and configuration 422/HTMX error and unavailable-theme messages without breaking their accessibility or focus.
- Assert the open menu and nested logout form stay within the menu and viewport, including keyboard navigation and logged-in/logged-out states; check Login separately as a page form.
- Cover system light/dark, explicit light/dark and Aurora at minimum, plus Firefox and WebKit. Include long-content stress cases and responsive snapshots where stable.
- Use an isolated test server, database, config, screenshot directory and test output. Do not invoke the default `tests/e2e/global-setup.ts` against a user's worktree: it writes and later deletes fixed `.e2e-config*` and `.e2e-screenshots` paths. The isolated fixture pattern in `tests/e2e/restart-fixture.ts` is a possible starting point after checking its cleanup behavior.

## Risks & Considerations

- Wider fields improve entry of long values but may make short numbers and primary/destructive actions disproportionately wide; preserve usable touch targets and validation visibility.
- Theme CSS is loaded after app CSS, followed by compatibility CSS (`web/templates/layouts/base.html:8`); verify final computed styles, not just declarations, across themes.
- Avoid horizontal overflow of long unbroken tag names and dynamically inserted content. Respect narrow viewports, keyboard focus, and screenshot image max-width.
- No migration or API change is expected for a scoped CSS-only fix; markup/class changes must also work in server-rendered and HTMX fragments.
- Existing tracked and ignored user files were not reset or cleaned. The measurements used disposable scripts inside this assessment directory, removed after use; no source files were changed.

## Open Questions

- [NEEDS CLARIFICATION: Should every page form, including Sign in and Delete, span the full list-item content width, or should some use the shared container while keeping shorter controls?]
- [NEEDS CLARIFICATION: Should short numeric fields, dropdowns, and action buttons stretch to that width or remain deliberately compact?]
