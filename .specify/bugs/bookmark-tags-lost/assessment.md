# Bug Assessment: Bookmark tags are lost when editing

- **Slug**: bookmark-tags-lost
- **Created**: 2026-11-02
- **Source**: pasted text
- **Verdict**: valid
- **Severity**: high

## Report (verbatim or summarized)

> If I edit a bookmark all tags are lost once I save the changes.

## Symptom

Saving an edit to a previously tagged bookmark through the PostgreSQL-backed web application removes its existing tag associations, even when the user changes only the description. Unchanged tags should remain associated with the bookmark; intentional additions and removals should be respected.

## Reproduction

The following is a code-derived reproduction; it has not been run in a browser during this assessment.

1. Sign in as an administrator and choose a bookmark that already has at least one tag.
2. Open its edit page. The current code loads the bookmark without its tags, so the tag list is empty.
3. Change only the description, leave the URL unchanged, and save. On the bookmarks page, the previously associated tags are gone.

The reporter's application version, deployment configuration, and exact edit steps are [NEEDS CLARIFICATION]. The code trace below applies to the current working tree and the PostgreSQL-backed web path.

## Suspected Code Paths

- `adapters/postgres/bookmark_maintain.go:24` — `FindBookmark` selects bookmark fields and screenshot storage key, but never loads `bookmark_tags` or assigns `bookmark.Tags`.
- `adapters/httpweb/maintain_handlers.go:26` and `bookmarker/usecase/update.go:28` — the edit handler renders the result of that maintenance lookup directly.
- `web/templates/pages/edit.html:14` — existing tag labels and hidden `tags` inputs are generated only from `.Bookmark.Tags`; an unpopulated slice produces neither.
- `adapters/httpweb/maintain_handlers.go:60` and `bookmarker/usecase/update.go:87` — the handler forwards submitted `tags`, and the use case normalizes them as the complete replacement set.
- `adapters/postgres/bookmark_maintain.go:52` — the update transaction deletes all existing associations, then recreates only submitted tags. An empty set therefore deletes all associations; adding new tags on the empty edit form would retain only those newly submitted.
- `adapters/postgres/bookmark_query.go:173` — the public browse/search store already hydrates display and normalized tag names from `bookmark_tags` and `tags`; this illustrates the data needed by the maintenance lookup.
- `web/static/js/bookmark-form.js:41` and `web/templates/pages/edit.html:14` — separately, remove listeners are attached only to newly created tag elements. Once existing tags are rendered, their server-rendered Remove buttons will not work without additional handling.
- `tests/e2e/us3-maintain-bookmarks.spec.ts:22` — the existing edit test checks the description and creation date, not tag preservation. `tests/integration/postgres_bookmark_maintain_test.go:37` tests direct replacement with supplied tags but does not check tag hydration by `FindBookmark`.

## Root Cause Hypothesis

**Confidence: high.** The maintenance read path leaves the embedded `model.Bookmark.Tags` empty even for a tagged bookmark. Consequently, the edit template submits no existing tag values, and the update path interprets that empty list as the entire desired replacement set and transactionally deletes the existing links. This is grounded in the code rather than an observed browser or database run. A separate UI gap means that merely loading the tags would expose nonfunctional Remove buttons for server-rendered tags.

## Proposed Remediation

**Preferred**: Extend PostgreSQL `FindBookmark` to load the bookmark's associated tag `display_name` and `normalized_name`, in deterministic normalized-name order, and return query/scan errors rather than silently presenting an empty tag list. A focused query by bookmark ID, or a suitably shared extraction of the existing browse hydration logic, would make the edit form render and submit the full current set. Keep the existing complete-replacement update semantics so a user can intentionally remove all tags.

Update the tag UI to support removal of both initially rendered and newly added items, for example by delegated click handling on the tag list, ensuring initialization remains safe for both direct page loads and HTMX swaps. Preserve normal HTML escaping and use DOM APIs for tag text. Do not silently substitute existing tags whenever the form submits an empty list: that would prevent intentionally clearing all tags.

**Files likely to change**:

- `adapters/postgres/bookmark_maintain.go`
- `web/static/js/bookmark-form.js`
- `web/templates/pages/edit.html` if a removal selector or other markup is needed
- `tests/integration/postgres_bookmark_maintain_test.go`
- `tests/e2e/us3-maintain-bookmarks.spec.ts`
- `tests/contract/http_maintain_test.go` if adding focused form/handler coverage

**Tests to add or update**:

- PostgreSQL integration: create a bookmark with multiple tags; verify `FindBookmark` returns both display and normalized names, including deterministic ordering, and verify an untagged bookmark returns an empty set.
- Browser end-to-end: edit only the description of a known tagged bookmark, verify tags are shown in the edit form and remain after saving and reloading, and verify tag filtering still finds the bookmark.
- Exercise adding a tag alongside existing tags, removing one pre-existing tag, and intentionally removing all tags; verify the exact persisted set after each save. Cover direct page loads and HTMX-swapped edit forms.
- Add HTTP/contract coverage for rendering existing hidden tag fields and posting multiple or no tags; verify explicit empty replacement remains possible. Retain the existing direct-store replacement test.

## Risks & Considerations

- **Data loss:** saving a tagged bookmark currently removes association rows. Tag records may remain, but those records alone do not identify which bookmark had each tag; recovery of lost associations may require a backup or other historical evidence.
- The tag query adds a database read to maintenance lookups, including update and delete. Ensure errors are propagated, resources are closed, and query cost remains bounded.
- Preserve full-replacement semantics for existing HTTP and JSONL callers (`adapters/jsonl/maintain_operations.go:22`); changing omission or empty-list semantics would require a separately designed compatibility change.
- Loading tags is not enough: pre-existing Remove buttons currently have no listener, so fix and test that UI path as well.
- Concurrent edits can still overwrite each other's tag sets under the existing replacement model; consider optimistic concurrency separately if required. No schema migration is necessary for the proposed fix.
- The working tree contained unrelated pre-existing modifications at assessment time; conclusions describe the current working tree, not necessarily a released version. No source changes or tests were made during this assessment.

## Open Questions

- [NEEDS CLARIFICATION: What application version and deployment does the reporter use, and were tags already absent when the edit form opened?]
- [NEEDS CLARIFICATION: Are backups or other historical records available for restoring already-lost bookmark-to-tag associations?]
