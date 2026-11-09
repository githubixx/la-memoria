# Web and Asset Contract

## Appearance and catalog

- Public full page loads (including `/login`, `/bookmarks`, `/search`, and administrator pages when authorized) use one non-secret effective appearance. `system` means no selected theme stylesheet and no explicit theme override; browser `prefers-color-scheme` changes apply while open. A valid catalog theme loads only its approved Chroma CSS/assets. Unknown persisted values or a config read failure render System rather than reflecting untrusted paths.
- Mount the pinned Chroma catalog handler under `/ui/`, using its documented prefix stripping. Its documented GET/HEAD allowlist, content types, cache revalidation and non-disclosure of metadata are preserved. Assets are delivered with the application, not downloaded at runtime. Missing artwork or fonts must not prevent use of content and controls.
- Base shell, menu, full pages, htmx fragments and feedback use shared Chroma tokens/components and Bookmarker compatibility styling. Preserve accessible labels, visible keyboard focus and mobile navigation, including every catalog option and System.

## Configuration

`GET /configuration` remains administrator-only, `Cache-Control: no-store`, and shows the existing redacted fields and a labeled `Theme` select. It lists System plus every bundled catalog option exactly once, with distinguishable labels. Select the saved ID or System for absent setting. If the stored ID is unavailable, show it clearly as unavailable in a selected sentinel and require a valid replacement.

`POST /configuration` remains administrator-only and requires existing same-origin/CSRF checks. It submits existing fields and a required `theme` string. The service validates the whole candidate and catalog membership before any commit. Success keeps the existing normal `303` redirect or htmx `200` with `HX-Redirect` to `/configuration` (including `restart_required=true` for database changes). The full navigation applies the saved theme to the entire current page without restarting the server. Other open pages may retain their prior appearance until full load.

Invalid, missing, empty, tampered or unavailable submitted IDs, invalid other fields, or pre-commit storage failure return a useful error and retain the prior effective page appearance and all saved settings. The existing validation failure status is `422`; make htmx render that form response explicitly instead of dropping it, preserving candidate inputs and accessible error association. A non-htmx error renders a full document with the persisted effective appearance, not the failed candidate's theme. Protect failure responses from caching and do not disclose secrets.

## Boundary tests

Exercise public asset GET/HEAD and invalid paths; authorization and CSRF; standard and htmx configuration submissions; all themes generated from catalog, reload/restart, System device changes, obsolete saved ID, invalid input, failing writer, and simultaneous database changes. Test public/admin journeys, keyboard focus, mobile, and missing decorative resources. Existing workflow paths and access restrictions remain unchanged.
