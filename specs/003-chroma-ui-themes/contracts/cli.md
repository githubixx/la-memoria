# JSON Lines Configuration Contract

The existing `bookmarker-cli` `v1` protocol remains one request per line on stdin and one JSON response per line on stdout; diagnostics go to stderr. Existing authenticated `configuration.get`, `configuration.validate` and `configuration.update` operations and error envelope remain in force. Unknown fields remain rejected, secrets redacted, and unauthenticated operations rejected.

## `configuration.get`

The redacted configuration result gains a top-level `theme` string. Return `system` for a missing YAML field; return a stored unavailable ID unchanged so the administrator can diagnose and repair it. Existing fields remain unchanged.

## `configuration.validate` and `configuration.update`

Payload remains a full editable configuration object for the existing fields; the optional top-level `theme` field is an exact ID or `system`.

| Input | Result |
|---|---|
| Omitted, stored valid | Preserve stored selection. |
| Omitted, no stored setting | Use System. |
| Omitted, stored unavailable | Reject; explicitly supply a valid replacement. |
| `system` or catalog ID | Accept subject to all other validation. |
| Empty, null, wrong type, or unknown ID | Reject with `validation_failed` (using existing error envelope); do not change any field. |

`validate` returns the normalized redacted candidate and `restart_required` without writing. `update` validates and commits the complete candidate atomically, then returns the same result shape. Failed validation or pre-commit I/O preserves the old file; after rename a directory durability warning is logged separately, not sent as a failed update. Database changes retain existing restart indication. Catalog membership is the same as web; System is not a bundled catalog entry.

## Compatibility and tests

Old clients omitting `theme` keep working without clearing a valid saved setting. Clients with strict response schemas must tolerate the new property; document this and unkeyed Go struct literal impacts in 0.3.0 migration notes. Contract tests use real JSONL encode/decode and the shared use case, including omission vs null, unavailable-value recovery, failed combined updates, authentication, and redaction. Integration tests exercise actual YAML read/write under owner-only permissions and cross-process serialization.
