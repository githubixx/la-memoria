# Bug Assessment: Theme menu readability over page content

- **Slug**: theme-menu-readability
- **Created**: 2026-10-30
- **Source**: pasted text and user-supplied screenshot (local, read-only)
- **Verdict**: valid
- **Severity**: medium

## Report (verbatim or summarized)

> Dark theme burger menu open over text has barely readable menu items. CRT theme works as expected. Other themes may be affected. User wants underlying background to show through a bit (about 10%) while menu items remain clearly readable.

The attached screenshot shows the Dark menu over the Configuration heading, select and input. Both the heading and form controls are visible through the menu, competing with its labels.

## Symptom

In Dark and System with a dark OS preference, the popup is almost transparent rather than a mostly opaque surface, allowing page content to interfere with menu labels. Other bundled themes have either similar transparency or a poor menu foreground/surface pairing. The expected result is readable navigation with roughly 10% of the underlying scene showing through, without dimming the menu labels themselves.

**Severity rationale**: navigation readability and accessibility are affected across many themes, including System-dark, but the menu remains operable and there is no evidence of data loss or a security issue. Findings describe the current worktree, which contains existing uncommitted theme integration changes; this assessment does not establish a released-version regression.

## Reproduction

1. Select the bundled Dark theme, or System while the operating system/browser prefers dark colors. Open a content-heavy page such as Configuration.
2. At a narrow viewport, open the burger menu where it overlaps the heading or form controls. Observe the content showing strongly through the labels. Compare with `crt-starfield-1`.
3. For other themes, repeat with the bundled theme selected. [NEEDS CLARIFICATION: affected browser versions and viewport sizes beyond the supplied screenshot.]

**Verification performed**: An isolated Playwright Chromium **headless** fixture at 403 × 403 CSS pixels, device scale 1, loaded the actual pinned Chroma 0.2.0 base/theme CSS, application CSS in production order, menu markup and menu JavaScript. A representative Configuration form and authenticated menu were used; no running production server, real browser session or user data was touched. Each of the 37 bundled theme stylesheets was measured with a light OS preference, plus System with both light and dark preferences. The fixture used actual CSS/assets and clicked the menu but was not a full application/server integration run; animations were disabled for reproducible sampling.

### Computed styles and sampled contrast

For contrast, the computed foreground color of each menu label was compared using the WCAG relative-luminance formula against the rendered screenshot pixels in the rectangular bounds of its text, after making only the menu foreground transparent. This preserves the actual composed background, gradients, images and underlying page text. The minimum over all five labels is shown below. These are **sampled worst-case background estimates**, not glyph-mask measurements or a full WCAG conformance audit: some sampled pixels may not lie under a painted glyph, other page content/positions may be worse, and anti-aliasing, focus/hover, animation, other browsers and viewports were not assessed. The 4.5:1 normal-text target is a useful diagnostic, not a claim that every label fails.

| Theme | Minimum sampled ratio |
|---|---:|
| aurora-glass-1 | 2.92:1 |
| cartoon-1 | 1.56:1 |
| crt-starfield-1 | 9.95:1 |
| dark | 1.29:1 |
| fitness | 1.43:1 |
| glas-1 | 1.00:1 |
| gold-1 | 7.46:1 |
| golden-moon | 5.33:1 |
| light | 5.70:1 |
| lighting-1 | 12.92:1 |
| luxury-1 | 8.98:1 |
| mantle-1 | 5.35:1 |
| metal-1 | 1.01:1 |
| retro-arcade | 11.64:1 |
| solar-system-1 | 6.24:1 |
| space-night-1 | 6.63:1 |
| spaceship-2 | 14.23:1 |
| spaceship-blue-dark / blue-light | 1.00:1 / 1.00:1 |
| spaceship-cyan-dark / cyan-light | 1.00:1 / 1.00:1 |
| spaceship-green-dark / green-light | 1.00:1 / 1.00:1 |
| spaceship-grey-dark / grey-light | 1.00:1 / 1.00:1 |
| spaceship-magenta-dark / magenta-light | 1.00:1 / 1.00:1 |
| spaceship-mono-dark / mono-light | 1.00:1 / 1.00:1 |
| spaceship-orange-dark / orange-light | 1.00:1 / 1.00:1 |
| spaceship-purple-dark / purple-light | 1.00:1 / 1.00:1 |
| spaceship-red-dark / red-light | 1.00:1 / 1.00:1 |
| spaceship-yellow-dark / yellow-light | 1.00:1 / 1.00:1 |
| System, light preference | 5.70:1 |
| System, dark preference | 1.29:1 |

Thus 26 of 37 bundled themes and System-dark have at least one sampled text rectangle containing pixels below 4.5:1 in this fixture; 20 of those bundled cases are the colored spaceship variants. This is a risk screen, not proof that each rendered glyph fails. CRT, System-light and the other cases above 4.5:1 in this particular layout are not guaranteed safe over arbitrary underlying content.

Representative computed results:

- **Dark and System-dark**: menu `background-color: rgba(0, 0, 0, 0)` with `background-image: linear-gradient(rgba(255,255,255,.06), rgba(255,255,255,.03))`; link `rgb(167,139,250)`. The gradient by itself allows approximately 94–97% of the underlying pixels through, rather than approximately 10%. Dark's “Sign out” rectangle ranged from 1.29:1 to 5.55:1, depending on the pixel behind it.
- **CRT**: menu `rgba(10,19,33,.96)`, no background image, link `rgb(245,182,59)`; sampled 9.95–10.39:1. Its surface passes roughly 4% through, not 10%, and visually explains the reported difference.
- **Light and System-light**: opaque white menu, `rgb(124,58,237)` link, 5.70:1. These currently show none of the underlying scene through the menu.
- **Aurora Glass**: a translucent multi-stop gradient and `rgb(139,92,246)` accent links; sampled 2.92–4.24:1. **Glas**: a pale gradient and pale accent `rgb(207,214,225)`; sampled 1.00–1.32:1. Simply adding opacity does not necessarily fix an unsuitable accent/foreground pairing.

## Suspected Code Paths

- `web/templates/layouts/base.html:8` — loads Chroma base, Bookmarker styles, optional selected theme, then compatibility styles; `data-theme` and the selected theme stylesheet are omitted for System.
- `bookmarker/usecase/appearance.go:29` — System and invalid/unavailable selections fall back to no selected stylesheet; Chroma's base media query therefore selects light or dark tokens.
- `web/templates/fragments/menu.html:2` and `web/static/js/bookmark-form.js:6` — Bookmarker uses its own `.site-menu` / `.menu-list` structure and behavior, not Chroma's `.nav-burger-menu`.
- `web/static/css/bookmarker.css:1` and `:72` — defines the initial menu background from `--bg-color`; `:95`–`103` colors links **and sign-out button** with the theme primary/accent token.
- `web/static/css/chroma-compat.css:22` — later `background: var(--surface-color, var(--bg-color))` overrides the whole menu background, including the initial solid color. A gradient is valid for `background`, so the fallback does not apply just because the gradient is translucent.
- Pinned dependency `github.com/githubixx/chroma-ui@v0.2.0`, `assets/css/base.css:50`–`60`, `:316`–`376` and `assets/themes/dark/theme.css:14`–`19` — provides System-dark's translucent gradient and separate Chroma burger rules. `assets/themes/crt-starfield-1/theme.css:10`–`13` supplies the much more opaque CRT surface.

## Root Cause Hypothesis

**Confidence: high for Dark/System-dark; medium for the full catalog.** Bookmarker maps Chroma's general-purpose surface token directly to its custom popup via the `background` shorthand, discarding the earlier solid backing. That token is not a guaranteed readable popup background: it can be a translucent gradient or a translucent/opaque color. Bookmarker also uses the primary accent for all menu labels, although that token is not necessarily designed for text on the chosen surface.

The evidence points primarily to **integration**, not simply an upstream Dark bug. In a separate headless computed-style comparison using Chroma's own `.nav-burger-menu`, pinned v0.2.0 Dark and System-dark have opaque `rgb(7,8,20)` beneath the gradient, `rgba(226,232,240,.96)` menu text, a scrim and blur. The Bookmarker menu does not match that selector or inherit its special backing. Chroma's own Aurora Glass, Glas and spaceship-blue-dark comparisons also produced different backed surfaces and/or text colors. Those comparisons establish different computed styles, not a full upstream contrast audit: other upstream themes or use cases may still have issues, and no upstream change has been investigated.

## Proposed Remediation

**Preferred**: Give Bookmarker's menu a dedicated, theme-aware background and foreground contract rather than treating `--surface-color` and `--primary-color` as a safe pair. Use a mostly opaque backing with approximately 10% transmission of underlying content, preserve the theme's character where possible, and select or override menu text and interactive-state colors together to meet normal-text contrast targets across the catalog. Consider a blur or local scrim to soften underlying *text*, but do not depend on blur alone for contrast. Preserve distinct readable focus, hover and active states for both anchors and the sign-out button.

Design layers carefully: `--surface-color` can be a gradient **or** a color, so using it blindly as `background-image` can invalidate a declaration, while a later `background` shorthand can reset the backing again. An additional opaque gradient or theme-specific overlay changes effective transmission; a 90% backing plus another semitransparent layer does not necessarily transmit exactly 10%. Image/gradient and moving content behind the menu mean a single `getComputedStyle().backgroundColor` contrast calculation is insufficient. Do not apply `opacity: .9` to the menu element: that also fades labels and descendants. Define safe fallback colors for unsupported CSS features and for missing or incompatible tokens. If strict contrast and a fixed 10% transmission conflict for a theme or content, prioritize readability and use a more opaque fallback; agree the visual trade-off before implementation.

**Alternatives**:

- Adapt Chroma's burger markup and theme-specific menu rules directly, retaining Bookmarker's accessibility and HTMX/sign-out behavior. This reduces duplicated styles but is a larger integration change; upstream's currently opaque Dark and some theme-specific behavior do not automatically meet the requested ~10% transmission, and all themes still need validation.
- Use a deliberately neutral light/dark popup pair with fixed contrast-safe text instead of preserving each theme's accent. This is simpler and easier to validate, but changes theme appearance.

**Files likely to change**:
- `web/static/css/chroma-compat.css` and/or `web/static/css/bookmarker.css` — menu-specific surface, foreground and states.
- `web/templates/fragments/menu.html` and/or `web/static/js/bookmark-form.js` only if a structural approach is chosen; preserve keyboard and form behavior.
- `tests/e2e/chroma-journeys.spec.ts` or a dedicated theme-menu Playwright suite; optionally `tests/contract/chroma_assets_test.go` for the integration contract.

**Tests to add or update**:
- Headlessly open the real application menu over headings, form fields and deliberately light, dark and high-frequency text/image backgrounds for every bundled theme and both System preferences. Use the actual catalog, not a frozen hand-maintained subset.
- Measure rendered/composited foreground and underlying background over actual glyph coverage, gradients and images; aim for at least 4.5:1 for normal menu text and validate link/button states, including focus and hover. Include dark/light extremes and an opaque fallback; retain screenshots for visual overlap and approximate transmission checks.
- Explicitly cover Dark, System-dark, CRT, Aurora Glass, Glas and colored spaceship variants. Check roughly 10% scene transmission without reducing text opacity; test scroll, clipping, focus, Escape, outside click, narrow and desktop viewports and supported headless browser engines.

## Risks & Considerations

- WCAG contrast and legibility depend on actual compositing and content behind the menu; a 90% alpha value alone is not a universal contrast guarantee, nor does it fully suppress underlying text.
- Background gradients, images, theme overrides, animation, stacking contexts, overflow clipping and browser support for color functions/backdrop filters can invalidate a naive implementation or test.
- Maintaining Chroma theme identity and existing CRT appearance may conflict with a uniform transmission target. Menu theme-specific overrides may need ongoing maintenance as the pinned catalog changes.
- Preserve existing dirty worktree changes. Assessment did not modify application source; no migration or API change is anticipated for a CSS-only solution.

## Open Questions

- [NEEDS CLARIFICATION: Is approximately 10% visibility intended for underlying page text and controls as well as decorative backgrounds, or should text be blurred/suppressed?]
- [NEEDS CLARIFICATION: Is a more opaque fallback acceptable where a theme cannot satisfy contrast at approximately 10% transmission?]
- [NEEDS CLARIFICATION: Which browser engines, operating systems and viewport sizes are officially supported?]
