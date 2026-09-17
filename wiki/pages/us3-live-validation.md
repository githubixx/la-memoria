---
title: US3 live validation
type: howto
sources: [S001, S008]
updated: 2026-09-02
---

# US3 live validation

The completed US3 implementation passed focused unit, contract, real-
PostgreSQL integration, formatting, vet, and build checks. (S001)

Run `npx playwright test tests/e2e/us3-maintain-bookmarks.spec.ts --project=chromium`
against the isolated PostgreSQL fixture to exercise the live maintenance flow.
(S001)

The passing Chromium scenario verifies sign-in, editing without changing the
rendered creation date, cancellation without mutation, confirmed deletion, and
the deleted bookmark's absence from both listing and search results. (S001)

This scenario also catches redirect parity regressions between htmx and
ordinary form requests; htmx success must navigate through `HX-Redirect`.
(S001)

The scenario runs within the shared, serial Playwright service fixture
described in [Playwright E2E lifecycle](./playwright-e2e-lifecycle.md). (S008)

For the persistence and file lifecycle that this flow exercises, see
[Screenshot maintenance](./screenshot-maintenance.md) and [HTTP bookmark
maintenance](./http-bookmark-maintenance.md). (S001)
