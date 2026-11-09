# Bug Fix: Theme menu readability over page content

- **Slug**: theme-menu-readability
- **Fixed**: 2026-10-30
- **Assessment**: ./assessment.md
- **Status**: applied

## Summary

Bookmarker now gives its custom popup a dedicated translucent surface and matching foreground instead of using Chroma's general surface and accent tokens as a menu color pair. The backing transmits approximately 10% of the scene without fading menu contents, while a small backdrop blur softens competing text where supported.

## Changes

| File | Change | Notes |
|------|--------|-------|
| `web/static/css/chroma-compat.css` | modified | Paired dark/light menu colors, CRT amber text, solid fallback, translucent backing, optional blur, link/button hover and focus states, and forced-colors treatment. |
| `tests/e2e/theme-menu.spec.ts` | added test | Headless real-application catalog, rendering, transmission, interaction and persisted-selection checks. |
| `tests/e2e/theme-menu.config.ts` | added | Isolated headless Chromium/Firefox configuration with unique retained output directories. |

## Tests Added or Updated

- `tests/e2e/theme-menu.spec.ts` — `all bundled menus remain readable over page content`: tests all 37 catalog themes plus System light and dark at 403 × 403 and 1280 × 800. It tests five actual authenticated menu labels including sign-out against both Configuration/theme backgrounds and an overlapping high-frequency black/white scene with heading, text and controls. It checks normal, link hover, sign-out hover, link keyboard focus and sign-out keyboard focus.
- The same test measures black-to-white transmission, exercises a no-blur fallback, verifies keyboard access and menu scrolling at 403 × 220, Escape, outside click, page scroll, sign-out, and persisted Dark, CRT, Aurora and System selection through the real application. It verifies successful loading of representative Aurora and CRT image assets.

## Local Verification

- `npx playwright test --config tests/e2e/theme-menu.config.ts --workers 2` → **2 passed**, Chromium and Firefox, headless, against isolated Bookmarker servers and local PostgreSQL containers.
- For each engine, 78 theme/preference/viewport cases were measured. The smallest sampled **rendered core-glyph pixel versus corresponding rendered background pixel** contrast was **7.25:1 in Chromium** and **7.16:1 in Firefox**, exceeding the test's 4.5:1 threshold. Each label also needed at least three detected painted core pixels. Contrast was computed using WCAG relative luminance, with screenshots taken before and after hiding only menu text.
- Actual compositing of uniform black versus white behind an unlabeled interior menu pixel measured **9.8% transmission** in both engines, within the test's 7–13% range. This is not a measurement for every pixel, state or backdrop; blur and other compositing affect local transmission.
- `npx tsc --noEmit` → passed; `git diff --check` → passed.
- Manual screenshot inspection: Dark over adversarial text and controls, plus Aurora and CRT over the real Configuration page. Screenshots from the final run are retained under `test-results/theme-menu-0b71970c-8797-4e63-903a-05875917cb4a/` (Chromium) and `test-results/theme-menu-e1f20af0-562b-4854-b598-3623d3d28514/` (Firefox); these are generated, ignored test outputs.
- Limitations: this is not a complete WCAG audit or a guarantee for every glyph, animation frame, page position, operating system, browser, high-contrast palette, or external theme. The pixel test excludes antialiased edge pixels and samples detected near-foreground glyph cores; three pixels per label cannot prove every character is legible. Animations were disabled; remote fonts were blocked by the application's CSP and tests used available fallback fonts. WebKit, forced-colors rendering, and an actual browser lacking support for translucent CSS were not exercised. The no-blur case was exercised, and the CSS includes an opaque fallback.

## Deviations from Assessment

- The menu uses a deliberately neutral, high-contrast dark pair for most themes and a light pair for Light/System-light; CRT retains amber text. This trades some theme-specific accent styling for a predictable foreground/background contract. Existing theme border and surrounding page styling remain.
- The catalog sweep dynamically loads each real server-served theme stylesheet in production order on the authenticated application page. Persisted configuration and server-rendered stylesheet selection are additionally exercised for Dark, CRT, Aurora and System, rather than saving all 39 selections.
- The dedicated Playwright config is an additional test file beyond the optional paths listed in the assessment; it isolates the fixture and headless browser configuration.
- Early verification used Playwright's default `test-results` output directory, which Playwright clears on startup; prior ignored results in that directory may have been removed. The final configuration uses unique directories so subsequent runs do not clear earlier results. Existing source worktree changes were not intentionally reverted.

## Follow-ups

- Run `/speckit.bug.test slug=theme-menu-readability` for independent verification.
- Consider adding WebKit, forced-colors and animated-background coverage when supported environments are defined; revisit theme-specific menu palettes if maintaining more accent identity is desired.
