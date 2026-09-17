---
title: HTTP bookmark maintenance
type: component
sources: [S001, S006, S007, S008]
updated: 2026-09-02
---

# HTTP bookmark maintenance

Edit and delete routes require an authenticated principal, and both mutation
handlers reject requests whose CSRF token does not verify against the session
token. (S006)

The update handler passes the authenticated principal's session ID to the
maintenance use case, which binds any submitted replacement capture to that
session and bookmark. (S006)

On validation or update failure, the edit handler reloads the bookmark and
returns the edit page with status `422`, preserving the submitted error as
renderable feedback. (S006)

For successful update and confirmed-delete submissions, ordinary requests use
`303 See Other` to `/bookmarks`, while htmx requests return `200 OK` with
`HX-Redirect: /bookmarks`. (S006)

The public bookmark result fragment renders Edit and Delete controls only when
its view model is authenticated. (S007)

The live Chromium run found that using `303` for htmx maintenance submissions
did not provide the required navigation behavior; switching those handlers to
`HX-Redirect` made the scenario pass. (S001)

The supported browser matrix uses an isolated, reset-before-each-test service
lifecycle; its setup and browser coverage are documented in [Playwright E2E
lifecycle](./playwright-e2e-lifecycle.md). (S008)

The file and transaction compensation behind those controls is documented in
[Screenshot maintenance](./screenshot-maintenance.md), with its end-to-end
check captured in [US3 live validation](./us3-live-validation.md). (S001)
