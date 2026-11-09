# Bug Fix: Aurora Glass background shifts between pages

- **Slug**: aurora-glass-background
- **Fixed**: 2026-10-30
- **Assessment**: ./assessment.md
- **Status**: applied

## Summary

Preserved Chroma's background properties by replacing the body background
shorthand with a color-only declaration. Reserved a stable scrollbar gutter
after confirming that a scrollbar appearing during real menu navigation
otherwise shifted the Aurora background even with fixed attachment.

## Changes

| File | Change | Notes |
|------|--------|-------|
| `web/static/css/bookmarker.css` | modified | Preserve background longhands and reserve the root scrollbar gutter. Pre-existing worktree edits were retained. |
| `tests/e2e/chroma-background.spec.ts` | added test | Exercise Aurora navigation, scrollbar threshold, scrolling, reload, unavailable resources, and System plus all 37 bundled themes. |
| `tests/e2e/chroma-background.config.ts` | added | Run the isolated fixture without the shared setup's fixed ports and cleanup of existing `.e2e-*` files; includes a mobile WebKit project. |

## Tests Added or Updated

- `tests/e2e/chroma-background.spec.ts` — `Aurora background stays fixed across menu navigation, scrolling, reload and asset failure`: compares a viewport background sample at short and tall page heights, checks scrollability and fixed attachment, tests reload and return navigation, verifies the image loaded, and repeats with decorative assets blocked. Exact pixels are required at rest; the scrolled comparison allows a mean channel difference below 1/255 for compositing variation.
- `tests/e2e/chroma-background.spec.ts` — `System and bundled themes retain Chroma backgrounds and fallback colors`: compares computed background longhands and nontransparent fallback colors with and without Bookmarker CSS for System and every bundled theme.

## Local Verification

- Before the fix, the headed Chromium Aurora test failed because attachment was `scroll` rather than `fixed`. With only the color change, Search → Configuration changed document client width from 1280 to 1265 pixels and the sampled background pixels differed. The stable gutter fixed the sampled visual shift; the test checks stable body width rather than `clientWidth`, which still reflects the presence of a scrollbar in Chromium.
- `npx playwright test --config tests/e2e/chroma-background.config.ts --project chromium --project firefox --project mobile --headed --output test-results/aurora-background-final` → **6 passed**: desktop Chromium, desktop Firefox, and iPhone-sized Chromium emulation, both tests each. The fixture used its own application and PostgreSQL container with an already-cached image; external font requests were blocked.
- `npx playwright test --config tests/e2e/chroma-background.config.ts --grep 'Aurora background' --headed --output .specify/bugs/aurora-glass-background/pw-aurora-final` → three projects passed; **mobile WebKit could not launch** because host dependencies `libicu74` and `libflite1` are missing. No system packages were installed. Generated diagnostics were moved to `test-results/aurora-background-diagnostics/`.
- `go test -tags=contract ./tests/contract/...` → passed.
- `go test ./bookmarker/...` → passed.
- `npx tsc --noEmit` → passed.
- `git diff --check` → passed.
- Manual checks: reviewed the existing worktree and confirmed the usual shared Playwright setup would overwrite existing `.e2e-*` files; it was not run. The previously shared reproduction server was not used.

## Deviations from Assessment

- Added `tests/e2e/chroma-background.config.ts` outside the listed likely files to avoid the shared test setup's fixed ports and destructive cleanup of existing fixture files. The regression test also uses an isolated fixture rooted in the workspace.
- Implemented the assessed conditional scrollbar remedy in `bookmarker.css`, rather than the optional compatibility stylesheet, because it is a shared root layout rule.

## Follow-ups

- Run the mobile WebKit project once its host dependencies are available, and verify on the reporter's actual browser and device; mobile emulation is not a physical-device test.
- Check older browsers without `scrollbar-gutter` support if they are within the supported browser policy. The catalog check verifies computed background properties, not visual baselines for every theme.
- Run `/speckit.bug.test slug=aurora-glass-background` for independent verification.
