# Bug Verification: Aurora Glass background shifts between pages

- **Slug**: aurora-glass-background
- **Tested**: 2026-10-30
- **Assessment**: ./assessment.md
- **Fix**: ./fix.md
- **Result**: partial

## Summary

The reported short-to-tall and tall-to-short menu navigation did not reproduce the background shift in headless Chromium, Firefox, or mobile Chromium emulation. Mobile WebKit could not launch because host libraries are missing, so its checks remain inconclusive; no fix-related regression was observed in the checks that ran.

## Checks Performed

| Check | Command / Action | Result | Notes |
|-------|------------------|--------|-------|
| Reproduction (post-fix) | Headless Playwright command below; Aurora navigation test | pass | Three browser projects selected and saved Aurora, compared background pixels across Search and Configuration at the same viewport and scroll position, and exercised scrolling, reload, and blocked assets. Mobile WebKit did not launch. |
| New / updated tests | `PWDEBUG=0 CI= GOPROXY=off GOSUMDB=off npx --no-install playwright test --config tests/e2e/chroma-background.config.ts --reporter=list --workers=1 --output .specify/bugs/aurora-glass-background/headless-verification-output` | fail | Exit 1: 6 tests passed in Chromium, Firefox, and mobile Chromium; 2 mobile WebKit tests could not start because `libicu74` and `libflite1` are missing. The failures are browser-launch errors, not assertions about the fix. |
| Regression suite | `GOPROXY=off GOSUMDB=off go test -tags=contract ./tests/contract/...` | pass | Exit 0; Go reported cached results. |
| Regression suite | `GOPROXY=off GOSUMDB=off go test ./bookmarker/...` | pass | Exit 0; Go reported cached results. The headless suite also checked System and all bundled themes in the three available projects. |
| Broader integration and shared e2e | Not run | skipped | The shared Playwright setup can overwrite existing `.e2e-*` files; broader integration and Compose tests were not run to avoid potentially destructive or network-dependent environment changes. |
| Lint / type-check | `npx --no-install tsc --noEmit` | pass | Exit 0; TypeScript check. No lint script is configured in `package.json`. |
| Whitespace | `git diff --check` | pass | Exit 0; checks tracked diffs, not untracked files. |

## Output Excerpts

```text
6 passed (2.3m)
2 failed [webkit-mobile]: browserType.launch: Host system is missing dependencies
  libicu74
  libflite1
PLAYWRIGHT_EXIT=1

ok github.com/githubixx/la-memoria/tests/contract (cached)
ok github.com/githubixx/la-memoria/bookmarker/model (cached)
ok github.com/githubixx/la-memoria/bookmarker/usecase (cached)
```

All browser tests were headless. The test fixture used a cached PostgreSQL image, isolated ports, and its own temporary application and database; HTTPS browser requests were blocked by the tests. The fixture cleaned up its temporary directories and container. Playwright diagnostics are under `./headless-verification-output/`.

## Residual Risks

- Mobile WebKit and the reporter's actual browser and device were not exercised. Mobile Chromium emulation is not a physical-device test.
- Older browsers without `scrollbar-gutter` support remain unverified. Theme-catalog tests compare computed properties, not visual baselines for every theme.
- The broader integration and shared e2e suites were skipped; Go regression results were cached.

## Recommendation

Hold final closure until the headless mobile WebKit checks can run with the missing host dependencies and the reporter's environment is checked where practical. The original symptom is resolved in the three available browser projects, with no observed fix-related regression.
