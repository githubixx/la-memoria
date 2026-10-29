# Bug Assessment: Upper-right burger menu missing across application views

- **Slug**: upper-right-burger-menu
- **Created**: 2026-10-29
- **Source**: pasted text, attached screenshot, and user-provided local reference application
- **Verdict**: valid
- **Severity**: medium

## Report (verbatim or summarized)

> Initially I said I want to have a burger menu at the upper right. I've pasted an example from a different application. You can find it in /opt/scripts/vdradmin-go/ directory.

The screenshot shows a burger icon at the upper right and an open, vertically stacked dropdown below it. The reference application's labels and dark styling are examples, not evidence that Bookmarker should gain TV-specific destinations or a new theme. No URL was supplied or fetched.

The slug was auto-generated after the user-availability response instructed autonomous operation. The destination directory did not exist before this assessment.

## Symptom

Bookmarker renders an inline navigation list on desktop and a text-only "Menu" toggle on narrow screens instead of the requested upper-right burger menu. On full-page views other than Add and Edit, the script containing the toggle handler is not loaded, leaving narrow-screen navigation collapsed without a working button handler.

Expected: every application view exposes the same upper-right burger control, opening a dropdown with List, Add, Search, Configuration, and the appropriate authentication action. This follows FR-001 and the user's screenshot clarification.

## Reproduction

1. Open `/bookmarks` at a viewport wider than `40rem`. The CSS hides `.menu-toggle` and displays `.menu-list` as inline flex navigation.
2. Set the viewport to 390 by 844 pixels and reload `/bookmarks`. The template displays a text "Menu" button rather than a burger icon; the narrow-screen CSS hides the navigation list.
3. Activate the button. A full-page List response loads htmx but not `bookmark-form.js`, so the menu click handler is not installed and the list cannot be opened through that button.
4. Compare the result with the supplied screenshot and FR-001 in `specs/001-bookmark-administration/spec.md:111`.

These are code-derived reproduction steps, not a recorded browser execution. No application server or browser suite was run during assessment. The desktop mismatch and missing script delivery are directly established by the source.

## Suspected Code Paths

- `web/templates/fragments/menu.html:3` - renders the literal text "Menu" in the toggle; contains the existing destination links, CSRF field, and authentication branches to preserve.
- `web/static/css/bookmarker.css:27` - the shared header already places the navigation at the right using flex layout.
- `web/static/css/bookmarker.css:40` - hides the toggle by default; `.menu-list` immediately below defaults to inline flex layout.
- `web/static/css/bookmarker.css:149` - restricts the toggle and right-anchored dropdown behavior to the `40rem` media query.
- `web/static/js/bookmark-form.js:7` - `setupMenu` owns click toggling, `.open`, `aria-expanded`, and Escape handling.
- `web/templates/layouts/base.html:9` - loads htmx, but no menu behavior script; the shared menu is rendered at line 14.
- `web/templates/pages/add.html:18` and `web/templates/pages/edit.html:19` - the only template script references to `bookmark-form.js` found in the application.
- `tests/e2e/accessibility-responsive.spec.ts:20` - checks a text-named Menu button only at 390px, without desktop burger, placement, dismissal, or cross-view assertions.
- `tests/e2e/accessibility-responsive.spec.ts:31` - the no-JavaScript check verifies a direct page and overflow, not navigation availability.

Reference application, outside this workspace:

- `/opt/scripts/vdradmin-go/web/templates/_nav.html:1` delegates to `chroma/header`.
- `/opt/scripts/vdradmin-go/internal/adapters/primary/http/navigation.go:19` supplies a `chromaui.Header` with `MenuLabel: "Menu"` and application-owned destinations, matching the screenshot's labels.
- `/opt/scripts/vdradmin-go/web/templates/index.html:13` loads the shared `/ui/js/menu.js`.
- `/opt/scripts/vdradmin-go/go.mod:8` declares `github.com/githubixx/chroma-ui v0.2.0`. The dependency's internal implementation was not inspected; no claim is made about its exact keyboard or dismissal behavior.

## Root Cause Hypothesis

**Confidence: high.** The navigation was implemented as a conventional responsive link bar rather than the always-available burger disclosure required by FR-001 and clarified by the report. Its markup uses a text label, while CSS makes disclosure behavior mobile-only. Separately, shared menu behavior is bundled into a form-specific script loaded only by Add and Edit, rather than initialized by every full-page layout. A cheap discriminating check is a fresh `/bookmarks` load at desktop and mobile widths: inspect the toggle's visibility and visual content, then activate it and inspect `aria-expanded` and the list's computed display. If the deployed application behaves differently, compare its served templates and assets with this checkout before changing the hypothesis.

## Proposed Remediation

**Preferred**: keep the current vanilla HTML/CSS/JavaScript approach and existing routes. Render an icon-only burger button with an accessible name such as "Menu" and a tooltip, anchored at the upper right on desktop and mobile. Make the collapsed dropdown the JavaScript-enhanced default at all supported widths. Preserve the existing header placement and theme; provide a right-aligned, vertically stacked panel with adequate width, stacking order, viewport containment, and scroll handling when needed. Do not copy the reference app's TV destinations or force its dark theme onto Bookmarker.

Deliver menu initialization through the shared layout on every view. The smallest initial approach is to load the existing script once from the base layout and remove duplicate Add/Edit inclusions, verifying that its form selectors remain safe. Ensure initialization remains correct after htmx content swaps: form-specific setup must still occur for newly inserted forms, and menu listeners must not accumulate. Extract a dedicated shared menu script only if needed to avoid coupling or repeated initialization.

Preserve `aria-controls`, synchronize `aria-expanded` with actual visibility, retain keyboard activation and Escape/focus return, and close on outside activation and navigation, including htmx navigation. Retain a usable no-JavaScript navigation fallback rather than hiding all destinations behind an inert button. Keep links as native navigation links, not an ARIA application-menu widget unless its full interaction contract is intentionally implemented.

**Alternative**: evaluate reuse of Chroma UI's header/menu if shared UI adoption is desired. The reference app already uses it, but introducing the dependency and integrating authentication forms/assets is broader than this fix; inspect its actual contracts and fallback behavior before choosing it. No dependency adoption is required to correct the local defect.

**Files likely to change**:

- `web/templates/fragments/menu.html`
- `web/static/css/bookmarker.css`
- `web/static/js/bookmark-form.js`
- `web/templates/layouts/base.html`
- `web/templates/pages/add.html`
- `web/templates/pages/edit.html`
- `tests/e2e/accessibility-responsive.spec.ts`

**Tests to add or update**:

- First add failing regression coverage, as required by the constitution, before changing application code.
- At desktop and mobile widths, assert that the upper-right burger button is visible, the enhanced dropdown starts collapsed, and activation displays the required destinations with matching `aria-expanded` state.
- Use a screenshot or explicit icon-markup assertion to distinguish the requested burger from a text-only button; accessible name "Menu" alone cannot discriminate them.
- Start from fresh full-page List, Search, sign-in, Add, Edit, and Configuration responses as permitted by authentication; verify consistent menu initialization rather than relying on prior page scripts.
- Verify keyboard activation, visible focus, Escape with focus return, outside activation, and closing after ordinary and htmx navigation.
- Verify authenticated and signed-out destinations, working sign-out/CSRF behavior, and preservation of protected-route enforcement.
- Verify the dropdown stays within desktop and narrow mobile viewports without clipping or unintended horizontal overflow, including long labels.
- With JavaScript disabled, verify navigation destinations remain reachable, not merely that a direct URL renders.
- Verify Add/Edit tag and capture setup still works after direct loads and htmx swaps if script placement changes. Use the existing Playwright Chromium, Firefox, and mobile projects for the affected scenarios.

## Risks & Considerations

- Severity is medium: the explicit navigation requirement is unmet across desktop views, and missing menu initialization can make mobile navigation unavailable. No data loss or backend security defect is evidenced.
- Moving form-script delivery can introduce duplicate handlers or break htmx-inserted form behavior; validate both direct and enhanced navigation.
- Hiding links unconditionally would preserve the current no-JavaScript accessibility problem. Gate collapsed behavior on successful enhancement or use an appropriate native disclosure approach.
- Dropdown positioning, stacking, focus indicators, and touch target sizing must work without obscuring unrelated controls or overflowing narrow screens.
- Preserve existing authentication checks and CSRF submission. This is a presentation and interaction fix, not an authorization change.
- No schema migration, API change, backend logging change, or new framework is expected. Exact visual fidelity to the reference theme is not established by the report.

## Open Questions

- [NEEDS CLARIFICATION: Is the reference intended only to specify burger placement and dropdown interaction, or also exact colors, rounding, and animation? Default to the existing Bookmarker theme.]
- [NEEDS CLARIFICATION: Does the deployed version reproduce the missing click handler on fresh List/Search loads? Source inspection supports it; browser reproduction remains outstanding.]
