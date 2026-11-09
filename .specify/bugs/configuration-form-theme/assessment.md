# Bug Assessment: Configuration text fields do not follow the theme

- **Slug**: configuration-form-theme
- **Created**: 2026-10-30
- **Source**: pasted text and user-supplied local screenshot
- **Verdict**: valid
- **Severity**: low

## Report (verbatim or summarized)

> The form fields are not looking very nice. The screenshot is from vdradmin-go where you can see that the form fields fit to the theme. That's mainly true for the configuration page. The other pages already implemented it correctly.

The supplied screenshot shows two vdradmin-go TLS text fields with rounded, dark, subtly shaded surfaces next to their labels. It does not identify the selected theme or demonstrate the current Bookmarker page.

## Symptom

On the configuration page, the seven text inputs with no explicit `type` render with browser-default control styling instead of the selected Chroma form styling. Numeric fields and selects do receive themed styles, so the same form is visibly inconsistent. Configuration text fields should follow the active theme, as explicitly typed text fields do on other pages.

The impact is mainly presentation and usability, across the configuration page and themes; no data loss or broken save behavior was established. Low severity reflects the confirmed visual defect, not a finding that every theme/control combination has sufficient contrast.

## Reproduction

1. In the current worktree, open the authenticated Configuration page with a theme selected, for example `aurora-glass-1`. [NEEDS CLARIFICATION: the theme in the supplied reference screenshot is unknown.]
2. Compare Page title or Database host with Database port or the Theme select. Compare a configuration text field with the explicitly typed Words field on Search.
3. The configuration text inputs have a small, square, browser-default appearance; the other controls use the theme's larger, rounded surface and colors.

**Verification performed:** A headless Chromium fixture used the actual configuration and search template markup and the pinned Chroma base CSS, Bookmarker CSS, selected theme CSS and compatibility CSS in the production order. It stripped Go template actions rather than running the application; consequently conditional notices and values in that fixture are not representative of a live page. Network resources were blocked, and no existing browser was used. All 37 bundled themes plus System light and System dark showed a difference between the implicit configuration text input and the explicit search text input.

For example, with `aurora-glass-1`, Chromium computed Page title at 22px high, 0px radius, no background image and 13.33px font; Words, Database port and Theme computed at 44px high, 8px radius, a gradient surface and 16px font. In the fixture screenshot the configuration text controls appeared as thin rectangular bars while the number and select controls were rounded and shaded. This is a qualitative comparison with the supplied reference, not a pixel match or proof of its theme identity.

An in-memory experiment setting `type="text"` on all seven configuration inputs made their computed background, border width/radius, color, font size, height and shadow match the search text input in Chromium for all 39 theme/System cases, and in Firefox for System light/dark, Aurora, Cartoon and Spaceship Blue Dark. At a 375px viewport these cases showed no document-level horizontal overflow. WebKit could not start because host libraries were missing. These fixture checks do not verify a live authenticated page, real theme switching, submission, HTMX, focus behavior, or every browser.

## Suspected Code Paths

- `web/templates/pages/configuration.html:5-30` — Page title, Favicon path, Screenshot root, Database host, Database name, Database user and Database password environment variable omit `type`; the browser still treats each as a text input, but attribute selectors do not match them. Number inputs explicitly specify their type and selects are elements matched independently.
- `web/templates/pages/search.html:3-7`, `web/templates/pages/add.html:4-11`, `web/templates/pages/login.html:4-10` — search, add and login use explicit text/password types; add/edit URL fields use `type="url"`. This supports the reported contrast with other pages, without establishing that every control on those pages is visually correct.
- `web/templates/layouts/base.html:8-11` — loads Chroma base, Bookmarker CSS, the optional chosen theme, then Bookmarker compatibility CSS.
- `web/static/css/bookmarker.css:181-193` — the older local control rule also selects explicit text/url/password types and selects, not inputs without a type attribute.
- `web/static/css/chroma-compat.css:8-19` — forces the form grid to one column and provides a local extension for URL and number inputs, but does not style untyped text inputs. Its focus-visible rule at lines 84–89 should remain effective.
- `go.mod:8`; pinned dependency `github.com/githubixx/chroma-ui@v0.2.0/assets/css/base.css:737-765,1176-1196` — Chroma's form and spaceship-specific styles use explicit attribute selectors such as `input[type="text"]`. The embedded catalog is exposed through `adapters/chromatheme/catalog.go`. This is a mismatch between valid implicit HTML text inputs and Chroma's selector contract, not evidence that the bundled assets failed to load.
- Reference project vdradmin-go, `web/templates/configurations.html:45-70` and `web/static/css/base.css:1257-1284` — the screenshot's TLS fields explicitly use `type="text"` and the reference grid CSS selects that attribute. Chroma's form styles derive from the same project; no reference code or assets need to be copied.

## Root Cause Hypothesis

**Confidence: high.** HTML defaults an input without a `type` attribute to text behavior, but CSS `[type="text"]` tests the actual attribute rather than that default behavior. The configuration template therefore bypasses both Chroma's themed form rules and the older local explicit-type rule. The same issue also bypasses Chroma's type-specific spaceship focus and shadow rules. The isolated browser measurements and the in-memory attribute change support this diagnosis. Chroma's narrower selectors could be generalized upstream, but the actionable defect in this application is the markup integration.

## Proposed Remediation

**Preferred**: Explicitly add `type="text"` to the seven currently untyped, visible configuration inputs in `web/templates/pages/configuration.html`. Preserve their IDs, names, values, labels, required flags and form/HTMX wiring. In particular, the database password *environment variable name* is configuration text, not the password value, so do not change it to a password control. Let existing Chroma selectors and tokens supply the theme-specific surface, radius, typography and spaceship effects; do not hard-code screenshot colors or copy reference CSS.

Do not treat the screenshot's two-column layout as an automatic requirement: the current app deliberately overrides `.form-grid` to one column and caps forms at 32rem. Matching the reference layout or increasing form width would be a separate design decision, with mobile and accessibility consequences.

**Alternative**: Add locally scoped `input:not([type])` rules that reproduce the applicable base and theme-specific behavior, or propose broader selectors upstream in Chroma. This is more fragile because it must cover both base styling and theme-specific states; a base-only override would miss spaceship effects. Upstream changes would require a separately managed dependency update and are not necessary for this fix.

**Files likely to change**:

- `web/templates/pages/configuration.html`
- `tests/e2e/chroma-journeys.spec.ts` or a focused new configuration-theme browser spec; optionally `tests/contract/http_configuration_test.go` for the rendered markup contract.

**Tests to add or update**:

- On the real authenticated page, check all seven configuration text inputs against a representative explicitly typed text control for themed computed surface/image, color, radius, size and appropriate shadow; use theme tokens and within-theme comparisons rather than assuming a single fixed color. Cover System light/dark, named light/dark, a gradient surface such as Aurora or Cartoon, and a spaceship theme; ideally iterate the bundled catalog using `tests/e2e/catalog/main.go`.
- Cover focus-visible/keyboard navigation, labels, mobile and desktop widths, long values, numeric fields and native selects. Include validation error and unavailable-theme rendering and the HTMX-swapped configuration form so those paths retain styling.
- Exercise a valid configuration save and invalid submission to ensure names, values, validation, CSRF and theme selection still work. Run browser coverage on Chromium and Firefox and WebKit where dependencies are available; avoid asserting that native dropdown popup rendering is identical across engines.

## Risks & Considerations

- Explicit text type preserves HTML input behavior and submission semantics, but regression-test configuration save and validation rather than assuming it.
- The CSS `background` token can be a gradient; using only `background-color` or fixed colors would not faithfully support the catalog. Browser-native select popups and forced-color modes require separate accessibility checks.
- Theme-specific style, contrast, focus and layout vary; the isolated style comparison is not an accessibility audit. External font/image loading was blocked, and no live application or screenshot baseline was tested.
- The current worktree already contains extensive unrelated changes. Assessment used those current files and did not modify source or reference projects.

## Open Questions

- [NEEDS CLARIFICATION: Which vdradmin-go theme is shown, and which Bookmarker theme/browser was observed?]
- [NEEDS CLARIFICATION: Is the requested outcome theme-consistent fields only, or also the reference's wide two-column configuration layout?]
