# Bug Assessment: Aurora Glass menu button glint

- **Slug**: aurora-menu-button
- **Created**: 2026-10-30
- **Source**: pasted text and supplied screenshot
- **Verdict**: likely valid, needs reproduction
- **Severity**: low

## Report (verbatim or summarized)

> There seems to be another bug for Aurora Glass 1 theme with the burger menu
> button on the upper right. The top of that button looks not like it should
> be IMHO. Looks like a corrupted 3D effect there.

The supplied screenshot shows the upper-right menu button with a bright, angular highlight near its
top edge. The report does not specify the expected design or browser.

## Symptom

With Aurora Glass 1, the menu button's top edge has a conspicuous,
angular glass-like highlight that the reporter perceives as a corrupted 3D
effect. The expected appearance is a coherent, intentional-looking menu
button; its exact desired treatment is not specified.

## Reproduction

1. Select Aurora Glass 1 as the application theme.
2. View the upper-right menu button and compare its top edge with the
   supplied screenshot.
3. [NEEDS CLARIFICATION: Confirm the reporter's browser, viewport, display
   scaling and whether the button was hovered or focused.]

An isolated **headless Chromium** page was constructed using the current
worktree's menu markup and application stylesheets, the pinned Chroma base
and Aurora stylesheet, and the bundled image assets, in application load
order. It did not exercise a running Bookmarker instance. External requests,
including the theme's font import, were blocked. The menu button measured
45 × 45 CSS pixels and the bundled `button-glint.svg` loaded successfully.
The computed `::after` used that image at `100% 100%`, screen blend, and
opacity 0.88. Temporarily hiding only that pseudo-element changed 476 pixels
in the top third of the 45 × 45 button screenshot (RGB channel difference
greater than 3); 395 and 309 pixels changed in the middle and bottom thirds.
This confirms the glint visibly contributes to the button, especially at
the top, but does not establish an exact pixel match to the reporter's
image or determine the intended visual design.

## Suspected Code Paths

- `web/templates/layouts/base.html:8-11` — loads Chroma base CSS, Bookmarker
  CSS, the chosen theme, then Bookmarker compatibility CSS.
- `web/templates/fragments/menu.html:3-4` — gives the square hamburger button
  both `menu-toggle` and the generic Chroma `btn` class.
- `web/static/css/bookmarker.css:44-61` — defines the button's dimensions,
  dark theme-derived background, border, rounded corners and hamburger bars.
- `web/static/css/chroma-compat.css:21` — raises its minimum dimensions to
  45 × 45 pixels.
- Chroma UI v0.2.0 `assets/css/base.css:452-467` — supplies generic button
  styling; the subsequent application `.menu-toggle` rule changes its
  background and dimensions.
- Chroma UI v0.2.0 `assets/themes/aurora-glass-1/theme.css:119-143` —
  applies an inset highlight and shadows to `.btn`, then overlays the SVG
  glint using a clipped, isolated `::after` pseudo-element. The later
  compatibility stylesheet does not override this decoration.
- Chroma UI v0.2.0
  `assets/themes/aurora-glass-1/assets/button-glint.svg:1-16` — has a
  100 × 40 viewBox with `preserveAspectRatio="none"`, a diagonal sweep, a
  spark, and a horizontal highlight stroke near its top. The theme stretches
  it to the almost-square button interior.

## Root Cause Hypothesis

**Confidence: high** that Aurora's generic `.btn::after` decoration is
rendered over this particular menu button and materially affects its top
pixels; **medium** that it is the precise feature the reporter calls
corrupted. The menu shares the generic button class, so the theme applies a
glint designed in a wide 100:40 SVG coordinate system across a 45 × 45
control, with the angular sweep and top stroke compressed horizontally and
stretched vertically. The theme's border and inset box-shadow also contribute
to the edge appearance. The SVG is an intentional bundled effect rather than
evidence of an asset-loading failure. The exact visual mismatch, its
acceptability, and any browser-specific contribution still need confirmation
against the reporter's environment and preferred design.

## Proposed Remediation

**Preferred**: After confirming the intended appearance, add a narrowly
scoped Aurora-specific compatibility rule for the menu toggle that suppresses
or replaces its generic glint with a highlight suited to a square icon
button. Preserve the theme's other button decorations, border, contrast,
focus indication and interaction behavior. Validate the border and inset
shadow separately before changing them; removing the glint alone is not yet
proven to meet the reporter's aesthetic expectation.

**Alternative**: Adjust the shared Aurora glint asset or theme selector in
Chroma UI to support square buttons. This may be more reusable upstream but
risks changing every themed `.btn` and requires coordinating the dependency
update.

**Files likely to change**:

- `web/static/css/chroma-compat.css`
- A focused test under `tests/e2e/`, for example
  `tests/e2e/chroma-selection.spec.ts`
- Potentially the upstream Chroma theme and asset, only if fixing the shared
  component instead of applying an application-specific override.

**Tests to add or update**:

- In a running application with Aurora Glass 1 selected, verify the menu
  button's default, hover, keyboard-focus and open states at desktop and
  mobile widths. Compare a stable button-region screenshot to an approved
  reference or assert the intended decoration and supplement it with visual
  review.
- Verify menu opening, keyboard operation and the visible focus indicator
  remain intact. Check System and other bundled themes, and ordinary Aurora
  buttons, for unintended appearance changes.
- Confirm graceful rendering when decorative SVG assets cannot load.

## Risks & Considerations

- The apparent defect is aesthetic and its intended design is not documented;
  removing all gloss may itself be an undesired theme change.
- A generic `.btn` or pseudo-element override could affect unrelated controls
  and themes; scope and cascade specificity matter.
- Screenshot rendering varies with browser, device scale factor, fonts,
  hover/focus state and background. The isolated check does not verify actual
  application navigation or the reporter's environment.
- No API, schema, data migration or security impact is expected from a
  targeted visual change.

## Open Questions

- [NEEDS CLARIFICATION: Which browser, version, viewport and device scaling
  produced the screenshot, and was the button hovered or focused?]
- [NEEDS CLARIFICATION: Should the button retain a subtler glass highlight or
  have no glint at all? Is there an approved reference appearance?]
