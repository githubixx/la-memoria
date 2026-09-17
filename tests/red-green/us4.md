# User Story 4 Red-Green Evidence

## Green Review - 2026-09-02

The configuration fixtures cover supported and unsupported favicon content,
alternate screenshot roots, reachable and unreachable database candidates,
protected defaults, and valid and invalid search limits.

| Check | Result |
| --- | --- |
| `npx playwright test tests/e2e/us4-configuration.spec.ts tests/e2e/security-boundaries.spec.ts tests/e2e/accessibility-responsive.spec.ts` | Pass, 36/36 across Chromium, Firefox, and mobile Chromium |

The US4 scenario verifies protected and redacted configuration, preserves
invalid submissions, rejects path/search/database candidates, activates a valid
candidate, follows the selected default view, and validates a configured
favicon response.