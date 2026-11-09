# Bug Verification: Bookmark tags are lost when editing

- **Slug**: bookmark-tags-lost
- **Tested**: 2026-11-02
- **Assessment**: ./assessment.md
- **Fix**: ./fix.md
- **Result**: partial

## Summary

PostgreSQL tag hydration and HTTP form/replacement tests passed, as did the selected regression and static checks. The original browser-and-database reproduction was not performed, so preservation through the complete edit journey remains unverified.

## Checks Performed

| Check | Command / Action | Result | Notes |
|-------|------------------|--------|-------|
| Reproduction (post-fix) | Run the assessment's sign-in, edit-description-only, save, and inspect-tags browser steps | skipped | The Playwright fixture deletes and rewrites existing `.e2e-*` data and truncates its database. Its fixed ports 54329 and 18080 were already occupied; running it could affect an existing service. |
| New PostgreSQL tests | `GOPROXY=off GOSUMDB=off TESTCONTAINERS_RYUK_DISABLED=true go test -tags=integration ./tests/integration -run 'TestPostgreSQLBookmarkMaintain' -count=1 -v` | pass | Exit 0; disposable local PostgreSQL containers; ordered tag hydration, untagged lookup, error propagation, and replacement. |
| New HTTP tests | `GOPROXY=off GOSUMDB=off go test -tags=contract ./tests/contract -run 'TestHTTPMaintain' -count=1 -v` | pass | Exit 0; escaped hidden tag fields, multiple tags, and clear-all replacement. |
| New browser test | `npx --no-install playwright test tests/e2e/us3-maintain-bookmarks.spec.ts --project=chromium` | skipped | Not run for the fixture and port safety reasons above; this also leaves real HTMX and browser removal behavior unverified. |
| Module regression suite | `GOPROXY=off GOSUMDB=off go test ./bookmarker/... ./adapters/postgres/... -count=1` | pass | Exit 0; model, use-case, and PostgreSQL adapter packages. |
| HTTP regression suite | `GOPROXY=off GOSUMDB=off go test -tags=contract ./tests/contract -run 'TestHTTP(Create\|Maintain)' -count=1 -v` | pass | Exit 0; six create and maintenance tests. The backslash in this table escapes the Markdown separator; the actual regex used an unescaped pipe. |
| PostgreSQL regression suite | `GOPROXY=off GOSUMDB=off TESTCONTAINERS_RYUK_DISABLED=true go test -tags=integration ./tests/integration -run 'TestPostgreSQLBookmark(Create\|Queries)' -count=1 -v` | pass | Exit 0; create and two query tests on disposable local containers; actual regex used an unescaped pipe. |
| Lint / type-check | `GOPROXY=off GOSUMDB=off go vet ./bookmarker/... ./adapters/postgres/... ./adapters/httpweb/...`; `npx --no-install tsc --noEmit`; `node --check web/static/js/bookmark-form.js` | pass | Each command exited 0. |
| Diff whitespace | `git diff --check -- adapters/postgres/bookmark_maintain.go web/static/js/bookmark-form.js web/templates/pages/edit.html tests/integration/postgres_bookmark_maintain_test.go tests/contract/http_maintain_test.go tests/e2e/us3-maintain-bookmarks.spec.ts` | pass | Exit 0. |

## Output Excerpts

```text
--- PASS: TestPostgreSQLBookmarkMaintainFindHydratesTags
--- PASS: TestPostgreSQLBookmarkMaintainUpdatesTagsAndPreservesCreationTime
ok   github.com/githubixx/la-memoria/tests/integration  2.950s

--- PASS: TestHTTPMaintainRendersAndReplacesTags
    --- PASS: TestHTTPMaintainRendersAndReplacesTags/multiple
    --- PASS: TestHTTPMaintainRendersAndReplacesTags/clear_all
ok   github.com/githubixx/la-memoria/tests/contract  0.121s

ok   github.com/githubixx/la-memoria/tests/contract  0.116s
ok   github.com/githubixx/la-memoria/tests/integration  4.385s
TypeScript exit: 0
JavaScript syntax exit: 0
```

## Residual Risks

- The assessed end-to-end symptom, real HTMX swap, and tag removal controls were not exercised in this verification. Passing the separate database and HTTP tests does not establish that the full browser journey works.
- Existing `.e2e-*` workspace data and occupied services were left untouched. Browser verification needs an isolated environment with disposable fixture data and unoccupied fixture ports.
- Previously lost associations are not reconstructed by this fix; concurrent full-replacement edits may still overwrite one another.

## Recommendation

Hold closure until the assessed browser reproduction and the new persisted-tag journey pass in a separate disposable environment. Do not run the current destructive fixture against existing workspace data or services.
