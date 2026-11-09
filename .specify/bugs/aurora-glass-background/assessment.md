# Bug Assessment: Aurora Glass background shifts between pages

- **Slug**: aurora-glass-background
- **Created**: 2026-10-30
- **Source**: pasted text
- **Verdict**: valid
- **Severity**: low

## Report

> The background image for Aurora Glass 1 theme behaves strange. No matter
> what menu item you select and switch to that page the background image
> looks different or is moved to a wrong position.

## Symptom

With Aurora Glass 1 selected, navigating between pages of different lengths
can change the background image's apparent crop or position. The expected
appearance is a stable background at the same viewport and scroll position.
The report does not specify a browser, viewport size, or exact menu sequence.

## Reproduction

1. Select and save Aurora Glass 1 in Configuration.
2. At a fixed viewport size, navigate between a short page such as Search and
   a substantially taller page such as Configuration, returning to the top
   of each page.
3. Compare the background image outside the page content.

The reported menu sequence was not exercised in a running Bookmarker instance.
An isolated Chromium page using the actual pinned Chroma base and Aurora CSS,
the Bookmarker stylesheets in production order, and the bundled desktop image
reproduced the underlying visual change. With the viewport and scrollbar width
held constant, changing only content height from 160px to 1600px changed a
50-by-50-pixel background screenshot. The computed body background attachment
was `scroll`. Temporarily setting it to `fixed` made that sampled region
identical across the two content heights. The image loaded successfully.

With scrollbar width *not* held constant, the document width changed from
1203px to 1188px between short and tall content; the sampled background
still changed even with fixed attachment. This is a separate potential source
of horizontal repositioning when scrollbars appear or disappear.

## Suspected Code Paths

- `web/templates/layouts/base.html:8` — loads Chroma base CSS, Bookmarker CSS,
  the selected theme, then compatibility CSS.
- `web/static/css/bookmarker.css:18` — the body `background` shorthand resets
  background longhands, including the attachment set by Chroma.
- Chroma UI v0.2.0 `assets/css/base.css:140` — sets body
  `background-attachment: fixed`.
- Chroma UI v0.2.0 `assets/themes/aurora-glass-1/theme.css:17` and `:54` —
  uses a centered, non-repeating background image with `cover` sizing and
  reapplies the image, size, position, and repeat, but not attachment.
- `web/templates/fragments/menu.html:7` — menu navigation swaps `#content`,
  whose height varies by page.
- `tests/e2e/chroma-selection.spec.ts` — checks theme selection, persistence,
  and keyboard behavior, but not the background's geometry across pages.

## Root Cause Hypothesis

**Confidence: high** for the CSS cascade and content-height effect. Bookmarker's
body `background` shorthand overrides Chroma's earlier fixed attachment with
the default `scroll`; the later theme stylesheet restores the image and
`cover` sizing but not the attachment. The resulting body-sized background
can be rescaled or recentered as page content height changes. Scrollbar-induced
viewport-width changes can additionally move a fixed background. The isolated
browser reproduction confirms both mechanisms, but the exact behavior in the
reporter's browser and actual navigation sequence remains unverified.

## Proposed Remediation

**Preferred**: Replace the body background shorthand in
`web/static/css/bookmarker.css` with a color-only declaration, preserving
Chroma's intended background attachment and other background properties.
Verify that the default and every bundled theme retain their existing colors
and images. If navigation across the scrollbar threshold still causes visible
movement, adopt a compatible stable-scrollbar or viewport-background approach
and verify it at desktop and mobile sizes rather than relying solely on
`background-attachment: fixed`.

**Alternative**: Restore `background-attachment: fixed` in the compatibility
stylesheet. This is narrower but leaves the shorthand resetting other Chroma
background properties and may not address scrollbar-induced movement.

**Files likely to change**:

- `web/static/css/bookmarker.css`
- `web/static/css/chroma-compat.css`, if needed for scrollbar or mobile behavior
- `tests/e2e/chroma-selection.spec.ts` or a focused theme-background test

**Tests to add or update**:

- With Aurora Glass 1 selected, compare a fixed background region after actual
  short-to-tall and tall-to-short menu navigation at the same viewport and
  scroll position, including the scrollbar threshold and a full reload.
- Assert the effective background image and attachment, with both assets loaded
  and decorative resources unavailable. Cover representative desktop and
  mobile browsers and check scrolling on long pages.
- Check System and other bundled themes for appearance regressions.

## Risks & Considerations

- Changing a shared body declaration can affect every theme, including System;
  verify the full theme catalog and fallback colors.
- Fixed background rendering and scrollbar behavior vary by browser and mobile
  platform. A single computed-style assertion is not sufficient visual proof.
- The isolated check deliberately blocked external fonts and did not connect
  to the user's deployment or modify application source.
- No API, schema, credential, or data migration impact is expected.

## Open Questions

- [NEEDS CLARIFICATION: Which browser, viewport size, and navigation sequence
  show the reported behavior most clearly?]
