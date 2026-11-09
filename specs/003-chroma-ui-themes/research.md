# Phase 0 Research: Chroma UI and Theme Selection

## Chroma source and integration

**Decision**: Pin `github.com/githubixx/chroma-ui v0.2.0` (tag commit `0de01b811e64e5814376f0b03a46dee23f7a9c30`) and use `chromaui.NewCatalog(nil)`, `Themes`, `Theme`, and its HTTP handler. Derive options and validation from this one catalog; do not enable filesystem overrides. Fail explicitly if catalog creation fails.

**Rationale**: The [Chroma README at v0.2.0](https://github.com/githubixx/chroma-ui/blob/v0.2.0/README.md) documents embedded assets, sorted catalog metadata, template components, CSS ordering and the reserved System mode. The [vdradmin-go go.mod](https://github.com/githubixx/vdradmin-go/blob/master/go.mod) pins v0.2.0, and its [theme manager](https://github.com/githubixx/vdradmin-go/blob/master/internal/infrastructure/theme/manager.go) demonstrates catalog lookup and HTTP integration. System is not a catalog entry. Metadata names and IDs, not a copied list or count, define the delivered options.

**Alternatives considered**: Copy static theme files or maintain a list: prone to drift and attribution maintenance. Local overrides: out of scope. Copy vdradmin-go's manager wholesale: its optional override lifecycle is unnecessary.

## Appearance and browser behavior

**Decision**: Render Chroma's base stylesheet, Bookmarker layout, selected-theme stylesheet if applicable, then compatibility CSS. In System omit explicit theme and selected-theme stylesheet; base CSS uses `prefers-color-scheme`, including live changes. On successful save use the existing full-page redirect (also `HX-Redirect`) rather than trying to swap only content.

**Rationale**: [Chroma's foundation contract](https://github.com/githubixx/chroma-ui/blob/v0.2.0/README.md) and [theme bootstrap](https://github.com/githubixx/chroma-ui/blob/v0.2.0/assets/js/theme.js) confirm System behavior and server-owned persistence. `web/templates/layouts/base.html` holds the shell and styles outside htmx's `#content`; replacing only that region cannot update the whole page. Use native accessible selects unless an enhancement proves equally accessible.

**Alternatives considered**: client-side localStorage or per-user theme: conflicts with deployment-wide selection. Dynamically swap stylesheets: adds transient failure/rollback and synchronization complexity. Force light or dark on load: fails device-change behavior.

## Configuration and commit boundary

**Decision**: Add YAML `theme`, expose it via redacted configuration, validate candidate membership in the shared use case through an injected catalog port, and distinguish absent CLI input from explicit invalid input. Loading is tolerant of unavailable stored IDs; updates are not. Preserve the prior stored value when omitted, then validate the resulting full candidate.

**Rationale**: `adapters/configyaml/load.go` uses strict known fields and `model.ValidateEditableConfiguration`, so rejecting an obsolete ID at load would prevent recovery. `ConfigurationService` serves both web and CLI. Its existing full-struct JSON decoding loses omitted-field presence; presence must be captured before merge. The persisted selection and effective System fallback must remain distinct.

**Alternatives considered**: validate only in HTML: CLI bypasses it. Silently replace invalid stored values: loses the evidence needed for explicit repair. Make every old CLI client send theme: violates preservation.

**Decision**: A successful atomic rename is the save commit point; all ordinary failure results occur before it. Sync the staged file, check short writes, protect its mode and open the directory before rename; log a post-rename directory sync failure as a durability warning rather than reporting rollback. Serialize writers across processes with a stable lock file and validate/merge inside the lock.

**Rationale**: `adapters/configyaml/update.go` currently returns errors from chmod, directory open and directory sync after rename, when the old bytes are already replaced. That contradicts FR-011. The CLI and web can independently use the same path, and the current service loads twice before activation. A bounded cross-process lock avoids stale interleaving. An fsync failure after rename means a successful running-state commit with uncertain power-loss durability, not a false failure claiming no change.

**Alternatives considered**: rollback on post-rename failure is itself fallible and cannot guarantee the original state; claiming failure while retaining the change is incorrect. A process-only mutex does not protect concurrent CLI and server writes.

## Compose persistence

**Decision**: Persist complete configuration in a dedicated owner-only named volume, initialize once, and atomically refresh only admin credentials from validated deployment inputs on startup using YAML-aware code. For an existing installation, export the old container's configuration before restart or recreation and provide it as a read-only file through the one-time `BOOKMARKER_CONFIG_IMPORT_PATH`. Validate and atomically import it into an empty volume, refreshing deployment-owned credentials. Fail explicitly on a missing, unreadable or invalid requested import, and refuse an import request against an initialized volume; remove the import setting and mount after successful migration. Initialize from the template only when no import is requested. Preserve all editable fields. Keep direct `BOOKMARKER_CONFIG` deployment behavior unchanged.

**Rationale**: `deploy/compose/entrypoint.sh` currently rewrites `/run/bookmarker/config.yaml` from the template at each start, while `docker-compose.yml` mounts no configuration volume. It cannot satisfy restart/recreation persistence, and mounting a new volume alone cannot capture edits from the old container. An explicit pre-upgrade export preserves recoverable settings and prevents an invalid migration from silently reverting to defaults. Credential refresh must not accidentally discard settings and must preserve the existing deployment-owned verifier policy.

**Alternatives considered**: persist theme in browser localStorage: not deployment-wide. Persist theme separately while continuing to reset other configuration: breaks combined settings expectations. Never refresh credentials after initial provisioning: silently breaks documented rotation behavior.

## Licensing, releases and verification

**Decision**: Retain Chroma's GPL-3.0-or-later notices and theme metadata/attribution, link its license, and avoid copying vdradmin-go implementation. Target Bookmarker 0.3.0 with migration notes for potentially breaking exported Go struct/strict JSON consumers while keeping JSONL `v1` request compatibility for omitted themes. The only released tag is 0.0.1; the 0.2.0 declaration in that tag was erroneous. Chroma was initially selected at v0.2.0 and subsequently upgraded to v0.3.0.

**Rationale**: [Chroma v0.2.0 license](https://github.com/githubixx/chroma-ui/blob/v0.2.0/LICENSE) and [attribution](https://github.com/githubixx/chroma-ui/blob/v0.2.0/README.md) describe GPL-3.0-or-later and individual bundled asset provenance; this repository's `LICENSE` is GPLv3. Adding exported fields can affect unkeyed Go literals and strict JSON response consumers. The constitution requires a minor increment and migration notes for breaking changes before 1.0.

**Alternatives considered**: claim patch compatibility despite response shape changes: unsafe. Copy reference project pages/assets without provenance: unnecessary. Run qualitative success criteria solely as automated checks: cannot establish user-study outcomes.

**Unresolved clarifications**: None. Theme catalog size is intentionally not pinned in this document; derive it from the exact selected module at implementation/test time. Verify asset notices during dependency packaging. Report participant-study results only after recruiting actual participants.
