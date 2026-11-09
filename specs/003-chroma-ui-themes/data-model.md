# Data Model: Chroma UI and Theme Selection

## Theme option

| Field | Type | Rules |
|---|---|---|
| `id` | string | Exact stable catalog ID; Chroma reserves `system`. Not a path from arbitrary user input. |
| `name` | string | Distinguishable display name from catalog metadata; disambiguate duplicate names with IDs if necessary. |
| `bundled` | bool | Must be true for delivered catalog options; no local overrides. |

One immutable process catalog supplies display options, exact validation and asset routes. `system` is an additional synthetic mode with display name “System”; it is never counted as a catalog theme. Catalog discovery failure is a startup error.

## Deployment configuration

| Field | Storage/interface | Rules |
|---|---|---|
| `theme` | Top-level YAML string; redacted JSON configuration property; form field `theme` | Missing YAML means effective `system`. Persisted `system` or catalog ID is valid. An unavailable persisted value remains readable and is preserved until explicitly replaced. |
| Existing fields | Existing YAML and redacted JSON | Preserve validation, credentials, permissions, and restart semantics. |

There is exactly one selection per shared configuration, not one per session. A new deployment defaults to System. The full editable configuration is a single transaction: no changes are persisted or activated if validation or pre-commit writing fails. Database connection changes continue to require restart; theme takes effect on the successful redirected page.

### Input presence and validation

JSONL `configuration.validate` and `configuration.update` retain the existing full configuration payload for other fields. If `theme` is absent, merge the stored theme before validation; if stored theme is missing, use `system`. If present, `theme` must be a nonempty string equal to `system` or a current catalog ID; null, empty strings, unknown IDs, and wrong types are invalid. An omitted or explicitly resubmitted obsolete stored ID fails validation and cannot save any other field. Web form submissions require a theme field and follow the same membership rules. Values are case-sensitive, without silent coercion.

### State transitions

1. **Missing** → visually System; a successful unrelated update persists `system`.
2. **Valid** → visually selected catalog theme or System. A valid successful update atomically commits the entire configuration and exposes the new theme on the next full render.
3. **Unavailable persisted ID** → visually System, retain original ID, show an unavailable selected sentinel on Configuration. Saving requires an explicit valid replacement.
4. **Invalid input or pre-rename write failure** → error; persisted configuration and prior effective appearance unchanged.
5. **Committed rename** → new complete configuration becomes authoritative. A subsequent directory-sync failure is logged as a durability warning, not represented as a failed/no-change save; crash persistence cannot be guaranteed in that exceptional case.

## Appearance projection

Derived per full render from configuration and catalog, without exposing other config values: `effective_mode` (`system` or valid catalog ID) and optional bundled theme metadata/stylesheet URL. An unavailable stored value or configuration-read failure projects to System; read failures are logged in structured, redacted form. Form presentation additionally needs the raw saved value and catalog options, but remains behind administrator authorization. Never construct asset URLs from an unvalidated submitted or persisted ID.

## Persistence invariants

- File contains no resolved database secret; existing verifier exception and 0600 restrictions remain.
- Preserve all noneditable settings, including administrator data and server configuration.
- Load, merge, validate and rename execute under a shared, stable cross-process lock; no reader observes a partially written document.
- Post-rename runtime reads use the newly committed complete file. Directory fsync warning is observable and does not masquerade as rollback.
- Compose persists the complete file across container restart/recreation, while startup refreshes only validated deployment-owned admin credentials.
- A one-time Compose import is permitted only into an empty configuration volume. Its source is a validated, read-only pre-upgrade export; an explicitly requested missing, unreadable or invalid source fails without creating a default configuration. Import and credential refresh commit atomically with mode 0600; an import request against an initialized volume fails without modifying it.
