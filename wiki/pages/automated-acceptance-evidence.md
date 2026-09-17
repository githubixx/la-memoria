---
title: Automated acceptance evidence
type: reference
sources: [S008]
updated: 2026-09-02
---

# Automated acceptance evidence

The recorded supported-browser matrix passed 78 scenarios across Chromium,
Firefox, and mobile Chromium; WebKit is excluded because the host lacked its
required Linux runtime libraries. (S008)

The PostgreSQL performance test seeds 100,000 bookmarks and enforces a
two-second limit for the first browse page and capped all-word search, while
separate query-plan assertions require index use. (S008)

Automated tests establish the exercised behavior for capture outcomes,
security denials, 50-page navigation, maintenance persistence/deletion, and
configuration redaction and activation; they do not establish population
rates or user satisfaction. (S008)

No representative first-time-user timing study or administrator feedback
survey was conducted. Consequently, SC-001, SC-002, SC-003's population-rate
component, and SC-008 remain unmeasured. (S008)

For fixture setup, isolation, and scope of the browser coverage, see
[Playwright E2E lifecycle](./playwright-e2e-lifecycle.md). (S008)
