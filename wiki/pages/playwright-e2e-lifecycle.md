---
title: Playwright E2E lifecycle
type: component
sources: [S008]
updated: 2026-09-02
---

# Playwright E2E lifecycle

The E2E global setup starts an isolated PostgreSQL 18.6 container, applies
migration `0001`, starts a loopback capture fixture and the Bookmarker web
process, then removes the temporary configuration and screenshots and stops
the processes at teardown. (S008)

An automatic fixture resets the runtime configuration and screenshot staging,
truncates mutable database tables, and reloads `bookmarks.sql` before every
test. (S008)

Playwright runs Chromium, Firefox, and an iPhone 13-sized Chromium project;
it uses one worker with parallel execution disabled because the lifecycle
shares fixed local ports and mutable service state. (S008)

The browser suites cover public browse/search and htmx parity, authenticated
capture/create/update/delete/configuration flows, security boundaries, and
no-JavaScript, keyboard, local-asset, and mobile-containment behavior. (S008)

The recorded results and limits of what this automation establishes are in
[Automated acceptance evidence](./automated-acceptance-evidence.md). (S008)
