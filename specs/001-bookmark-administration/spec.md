# Feature Specification: Bookmarker Administration

**Feature Branch**: `initial`

**Created**: 2026-08-31

**Status**: Draft

**Input**: User description: "Build the Bookmarker administration web application for public bookmark browsing and search, authenticated bookmark management, screenshot capture, pagination, and configurable branding, storage, database, and defaults."

## Clarifications

### Session 2026-08-31

- Q: How should Bookmarker obtain the administrator password while keeping authentication configured through `config.yaml`? → A: Store the username and a salted one-way password hash in `config.yaml`.
- Q: Which network destinations may Bookmarker access when capturing a bookmark screenshot? → A: Allow public and private HTTP/HTTPS destinations.
- Q: What should Bookmarker do after repeated failed administrator sign-in attempts? → A: Block the username/source-address pair for 15 minutes after 5 failures within 10 minutes.
- Q: When should an authenticated administrator session expire? → A: Expire when the browser session closes.
- Q: Should authenticated administrators be able to change the administrator username or password from the Configuration view? → A: Neither credential is editable in the web view.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Browse and Find Bookmarks (Priority: P1)

As a visitor, I can open Bookmarker, browse saved bookmarks, filter by a selected tag, and search tags or description words without signing in so that the collection remains useful as a public reference.

**Why this priority**: Reading and finding bookmarks is the core value of the application and must remain available to every visitor.

**Independent Test**: Populate more than one page of bookmarks, open the application while signed out, and verify list ordering, bookmark details, tag filtering, free-text search, result limits, and pagination without attempting any administrative action.

**Acceptance Scenarios**:

1. **Given** bookmarks exist and the configured default view is List, **When** a visitor opens Bookmarker, **Then** the first page lists the newest bookmarks first and shows each bookmark's URL, description, creation date, and tags in that order.
2. **Given** more bookmarks exist than fit on one page, **When** a visitor browses the list, **Then** the page shows Previous, a bounded sequence of page links with an ellipsis when needed, the final page, and Next centered at the bottom.
3. **Given** a bookmark has a tag, **When** a visitor selects that tag from the List view, **Then** the List view shows only bookmarks carrying that exact tag and provides pagination when needed.
4. **Given** matching bookmarks exist, **When** a visitor searches by tag, words in the description, or both, **Then** matching results are returned within the configured per-page and maximum-result limits.
5. **Given** a visitor is not signed in, **When** they browse or search, **Then** they can use those functions but do not receive Edit or Delete controls.

---

### User Story 2 - Add a Bookmark with Tags and Screenshot (Priority: P2)

As an authenticated administrator, I can enter a URL, description, and tags, review a captured screenshot, adjust the tag list, and save the bookmark so that new references are complete and visually recognizable.

**Why this priority**: The collection cannot grow without a controlled way to add complete bookmark records.

**Independent Test**: Sign in with the configured administrator credentials, enter a valid URL, description, and several tags, remove one tag before saving, verify the screenshot preview, save, and confirm the new bookmark appears in the list.

**Acceptance Scenarios**:

1. **Given** an authenticated administrator is on the Add view, **When** they enter a valid URL, **Then** Bookmarker captures the destination page and displays a screenshot preview before the bookmark is saved.
2. **Given** the administrator enters a tag, **When** they add it, **Then** the tag appears in a list below the tag input and can be removed from that list before saving.
3. **Given** a valid URL, non-empty description, and at least zero tags, **When** the administrator saves the bookmark, **Then** Bookmarker stores the bookmark, its creation date, its remaining tags, and the captured screenshot, and confirms success.
4. **Given** screenshot capture fails, **When** the failure is reported, **Then** the administrator can retry capture or save the bookmark without a screenshot and the failure is clearly identified.
5. **Given** a visitor is not signed in, **When** they try to open Add, **Then** Bookmarker requires a successful sign-in before displaying bookmark creation controls.

---

### User Story 3 - Maintain Existing Bookmarks (Priority: P3)

As an authenticated administrator, I can edit or delete bookmarks from the List view so that incorrect, outdated, or unwanted entries can be maintained.

**Why this priority**: Administrative maintenance preserves the accuracy and usefulness of the collection after creation.

**Independent Test**: Sign in, edit every editable bookmark field, verify URL changes trigger a new screenshot opportunity, then delete a separate bookmark after confirmation and verify it no longer appears in browse or search results.

**Acceptance Scenarios**:

1. **Given** an administrator is signed in, **When** they open List, **Then** each bookmark includes Edit and Delete controls.
2. **Given** an administrator edits a bookmark, **When** they save valid changes, **Then** its URL, description, tags, and screenshot state reflect those changes without changing its original creation date.
3. **Given** an administrator changes a bookmark URL, **When** the new URL is accepted, **Then** Bookmarker captures and previews a replacement screenshot before the changes are saved.
4. **Given** an administrator chooses Delete, **When** they confirm the destructive action, **Then** the bookmark and its associated screenshot are removed from Bookmarker and no longer appear in list, tag-filter, or search results.
5. **Given** an administrator chooses Delete, **When** they cancel the confirmation, **Then** the bookmark remains unchanged.

---

### User Story 4 - Customize Bookmarker (Priority: P4)

As an authenticated administrator, I can configure branding, the initial view, storage, database access, and search result limits so that Bookmarker fits its deployment without source changes.

**Why this priority**: Customization makes one application suitable for different deployments but depends on the core browsing and management workflows.

**Independent Test**: Sign in, update each supported setting in Configuration, restart or reload Bookmarker as directed, and verify the title, favicon, default view, screenshot location, database connection, and search limits take effect while invalid settings are rejected.

**Acceptance Scenarios**:

1. **Given** an authenticated administrator opens Configuration, **When** they save a valid page title, favicon, or default view, **Then** the updated branding and initial view are used on subsequent page loads.
2. **Given** an authenticated administrator saves valid screenshot storage and database settings, **When** Bookmarker subsequently stores and retrieves a bookmark, **Then** it uses the configured locations.
3. **Given** an authenticated administrator saves valid search page-size and maximum-result settings, **When** a search is performed, **Then** those limits govern the displayed and total returned results.
4. **Given** an invalid or incomplete setting, **When** an administrator attempts to save Configuration, **Then** Bookmarker identifies the invalid setting and preserves the last valid configuration.
5. **Given** a visitor is not signed in, **When** they try to open Configuration, **Then** Bookmarker requires a successful sign-in before revealing or allowing changes to configuration values.

### Edge Cases

- An empty bookmark collection shows a clear empty state rather than pagination controls.
- Invalid, unsupported, unreachable, redirecting, or slow URLs produce a clear capture status and cannot leave a partially created bookmark; non-HTTP/HTTPS schemes are rejected.
- A URL that already exists is accepted as a separate bookmark so users can preserve different descriptions or tag sets.
- Empty tags are ignored; duplicate tags within one bookmark collapse to one value; tag comparison ignores letter case and surrounding whitespace while preserving a consistent display form.
- Descriptions containing punctuation, mixed case, or multiple searched words are matched without requiring exact capitalization; an empty search is rejected with guidance instead of returning the entire collection.
- Requests for page numbers below 1 or beyond the last page resolve to a valid available page without exposing an error page.
- When search matches exceed the configured maximum, Bookmarker clearly indicates that results were capped.
- If a screenshot file is missing or unreadable, bookmark details remain available and a placeholder is shown instead of a broken image.
- Closing the browser session, signing out, or presenting an invalid session removes administrative capabilities and requires sign-in again before a write operation can complete.
- Five failed sign-in attempts for the same username and source-address pair within 10 minutes trigger a 15-minute block for that pair; other source addresses are unaffected.
- A favicon with an unsupported format or unusable content is rejected while the previous favicon remains active.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Bookmarker MUST provide List, Add, Search, and Configuration destinations through a menu opened from the top-right of every application view.
- **FR-002**: Bookmarker MUST open the configured default destination when a user visits the application root, using List when no other valid default has been configured.
- **FR-003**: Visitors MUST be able to use List, tag filtering, and Search without authentication.
- **FR-004**: Bookmarker MUST authenticate administrative users by comparing the supplied password with the configured salted one-way password hash before permitting bookmark creation, editing, deletion, or configuration access.
- **FR-005**: Bookmarker MUST prevent unauthenticated users from viewing protected configuration values or completing any administrative operation, including through a direct request.
- **FR-006**: Authenticated users MUST be able to end their session, after which administrative controls and operations are unavailable until they sign in again.
- **FR-007**: The List view MUST order bookmarks from newest to oldest and display, in order, the URL, description, creation date, and tags for each entry.
- **FR-008**: Bookmark URLs MUST be selectable and open their destination without replacing the user's current Bookmarker view.
- **FR-009**: The List view MUST display 10 bookmarks per page.
- **FR-010**: When multiple list pages exist, Bookmarker MUST display centered pagination at the bottom with Previous on the left, Next on the right, the current page clearly distinguished, the first and final page reachable, and an ellipsis where the full page range is omitted.
- **FR-011**: Selecting a bookmark tag MUST open the List view filtered to bookmarks carrying that exact normalized tag.
- **FR-012**: Search MUST support a tag criterion, a description-word criterion, or both; when both are supplied, returned bookmarks MUST satisfy both criteria.
- **FR-013**: Description search MUST be case-insensitive and match bookmarks containing all entered words regardless of word order.
- **FR-014**: Search results MUST be newest first, paginated according to the configured search page size, and capped at the configured maximum number of results.
- **FR-015**: Authenticated users MUST be able to create a bookmark with a valid web URL, a non-empty description, and an optional set of tags.
- **FR-016**: Adding a non-empty tag during creation or editing MUST place it in a visible list below the tag input, and users MUST be able to remove any listed tag before saving.
- **FR-017**: A bookmark MUST NOT contain duplicate normalized tags, and tag matching MUST ignore case and surrounding whitespace.
- **FR-018**: Entering or changing a valid public or private HTTP/HTTPS URL MUST initiate screenshot capture and display a preview or an in-progress status in the bookmark form; other URL schemes MUST NOT be captured.
- **FR-019**: If capture fails, Bookmarker MUST explain the failure and allow an authenticated user to retry or explicitly continue without a screenshot.
- **FR-020**: Saving a new bookmark MUST record its URL, description, creation date, tags, and screenshot reference when capture succeeded.
- **FR-021**: Authenticated users MUST receive Edit and Delete controls for each bookmark in the List view.
- **FR-022**: Authenticated users MUST be able to edit a bookmark's URL, description, and tags; changing the URL MUST offer a replacement screenshot, while the original creation date remains unchanged.
- **FR-023**: Deletion MUST require confirmation and, once confirmed, remove the bookmark from all browse and search results and remove its associated screenshot.
- **FR-024**: All deployment configuration MUST be represented in a single `config.yaml` file, including the administrator username and salted one-way password hash, page title, favicon, default view, screenshot storage location, database settings, search page size, and maximum search results.
- **FR-025**: Bookmarker MUST never store the administrator's plaintext password in `config.yaml`, display the configured password hash, or expose supplied passwords or the hash in user-visible errors or operational records.
- **FR-026**: The Configuration view MUST allow authenticated users to change the page title, favicon, default view, screenshot storage location, database settings, search page size, and maximum search results.
- **FR-027**: Configuration changes MUST be validated before activation; failed validation MUST preserve the last valid configuration and identify which setting needs correction without exposing secrets.
- **FR-028**: Bookmarker MUST use a valid configured title and favicon throughout the application and MUST provide usable defaults when either is not configured.
- **FR-029**: Bookmarker MUST preserve bookmark data across application restarts using the configured database and preserve screenshots in the configured filesystem location.
- **FR-030**: Bookmarker MUST provide clear success, validation, authentication, capture, configuration, and destructive-action feedback without losing valid user-entered form data.
- **FR-031**: After 5 failed sign-in attempts for the same username and source-address pair within 10 minutes, Bookmarker MUST reject further sign-in attempts for that pair for 15 minutes and MUST automatically permit attempts again after the block expires.
- **FR-032**: An administrator session MUST expire when its browser session closes and MUST become invalid immediately when the administrator signs out.
- **FR-033**: The Configuration view MUST NOT allow the administrator username or password hash to be viewed or changed; credential rotation MUST be performed by updating `config.yaml` through the deployment environment.

### Key Entities

- **Bookmark**: A saved web reference with a URL, description, immutable creation date, zero or more tags, and an optional screenshot reference.
- **Tag**: A normalized label associated with one or more bookmarks and used for exact filtering and search.
- **Screenshot**: A captured visual representation associated with one bookmark, including its storage reference and capture status.
- **Administrator Session**: Browser-session-scoped authenticated state that grants bookmark and configuration management capabilities until the browser session closes, the administrator signs out, or the session becomes invalid.
- **Application Configuration**: The deployment-wide settings for credentials, branding, default destination, screenshot storage, database access, and search limits.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: At least 90% of first-time visitors can find a known bookmark by browsing, selecting a tag, or searching its description in under 60 seconds without assistance.
- **SC-002**: At least 90% of authenticated users can add a bookmark with a reviewed screenshot and tags in under 2 minutes on their first attempt.
- **SC-003**: For at least 95% of reachable URLs under normal operating conditions, users see either a screenshot preview or a clear capture failure within 15 seconds of entering the URL.
- **SC-004**: At least 95% of browse, tag-filter, and search actions display their requested page within 2 seconds for collections of up to 100,000 bookmarks and configured result caps of up to 10,000.
- **SC-005**: In acceptance testing, 100% of unauthenticated attempts to create, edit, delete, or configure data are denied without changing or revealing protected data.
- **SC-006**: For collections spanning 50 or more pages, users can reach the first, adjacent, selected, and final pages with no more than two pagination selections from the currently displayed pagination control.
- **SC-007**: In acceptance testing, every successful bookmark edit is reflected in subsequent list and search results, and every confirmed deletion removes the bookmark and screenshot with no orphaned user-visible entry.
- **SC-008**: At least 90% of administrators rate the clarity of configuration validation and bookmark-management feedback as 4 or better on a 5-point scale.

## Assumptions

- Bookmarker starts with one configured administrative identity; self-registration, password recovery, multiple roles, and per-user bookmark ownership are outside this feature.
- `config.yaml` stores the administrator username and a salted one-way password hash; the plaintext password is supplied only during authentication and is never persisted or logged.
- Administrator username and password-hash changes are deployment-managed and are outside the web Configuration workflow.
- Authentication does not persist across browser sessions; closing the browser session requires the administrator to sign in again.
- List is the initial default view. Configuration may change the default to List, Add, Search, or Configuration; protected defaults require sign-in before their content is shown.
- The menu destinations remain visible to visitors so the application's structure is predictable, but Add and Configuration require sign-in before protected content is displayed.
- Bookmark descriptions are plain text. Rich-text editing, full-page content indexing, browser import, bookmark export, and automated link-health monitoring are outside this feature.
- Search combines tag and description criteria with AND semantics, while all words within the description criterion must be present.
- Duplicate URLs are allowed because separate entries may intentionally carry different descriptions or tags.
- Screenshot capture may access public and private HTTP/HTTPS destinations and targets the final destination after ordinary redirects. Sites that block capture or cannot be reached may be saved without a screenshot after explicit confirmation.
- List pagination remains fixed at 10 entries per page; only search page size and maximum search results are configurable in this feature.
- Configuration changes are deployment-wide and become active only after successful validation. Settings that cannot safely change during an active session may clearly request an application restart.
- Dates are presented in a consistent human-readable form appropriate to the deployment's configured locale and time zone.
