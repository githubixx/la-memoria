# CLI JSON Lines Contract

**Protocol version**: `v1`  
**Feature**: [Bookmarker Administration](../spec.md)  
**Model**: [data-model.md](../data-model.md)

## Process Contract

`bookmarker-cli` reads UTF-8 JSON Lines from stdin and writes UTF-8 JSON Lines to stdout. It processes requests sequentially: one complete request line produces exactly one complete response line before the next request is handled. Blank lines are ignored and never produce output.

- Machine responses are the only stdout content.
- JSON `slog` diagnostics use stderr and never contain password, verifier, session/CSRF material, resolved environment values, database credentials, form bodies, or sensitive full URLs.
- Per-request domain or validation failures do not terminate the stream.
- Clean EOF after processed requests exits 0. Configuration, database, migration, or protocol initialization failure exits nonzero after a safe stderr record.
- A bounded maximum line size prevents unbounded allocation. Oversized lines produce `line_too_large` when recoverable or terminate with a safe diagnostic when framing cannot continue.
- Unknown fields, unknown operations, trailing JSON values, unsupported versions, and wrong JSON types are rejected. Payload field errors use stable paths.

## Request Envelope

```json
{"version":"v1","id":"req-001","operation":"bookmark.list","payload":{"page":1}}
```

| Field | Type | Required | Rules |
| ------- | ------ | ---------- | ------- |
| `version` | string | Yes | Exactly `v1`. |
| `id` | string | Yes | Non-empty caller correlation ID within a documented size bound; echoed unchanged. |
| `operation` | string | Yes | One operation from the registry below. |
| `payload` | object | Yes | Strict operation-specific object; use `{}` when empty. |

## Response Envelopes

Success:

```json
{"version":"v1","id":"req-001","ok":true,"result":{"page":1,"items":[]}}
```

Failure:

```json
{"version":"v1","id":"req-001","ok":false,"error":{"code":"validation_failed","message":"request contains invalid fields","fields":{"payload.url":"must be an absolute HTTP or HTTPS URL"},"retryable":false}}
```

| Error field | Type | Meaning |
| ------------- | ------ | --------- |
| `code` | string | Stable machine-readable category. |
| `message` | string | Safe summary; never echoes secrets or sensitive full URLs. |
| `fields` | object | Optional field-path to safe validation message map. |
| `retryable` | boolean | Whether retry can succeed without changing request semantics. |
| `retry_after_seconds` | integer | Optional bounded delay for throttling or capacity errors. |

Malformed JSON without a readable `id` returns `id: null`. Duplicate request IDs are allowed because correlation ownership belongs to the caller; each line still receives its own response.

## Stream Authentication

The CLI is a local delivery adapter but protected use cases still require administrator authentication. Authentication belongs to one running stdin/stdout stream:

1. `auth.sign_in` accepts the configured username and plaintext password over stdin.
2. The adapter invokes the same authentication and throttle use case as HTTP with a local transport source identity.
3. Success creates a server-side session context held only in process memory and returns `{"authenticated":true}`. No reusable session token is written to stdout.
4. Subsequent protected operations on that stream carry the in-memory principal to the same authorization checks used by HTTP.
5. `auth.sign_out`, failed session validation, EOF, signal termination, or process exit revokes and clears the stream session.

Password input is never repeated in a response or diagnostic. A blocked local pair returns the same generic failure category and retry delay as HTTP. Public browse and search operations work before sign-in.

## Operation Registry

| Operation | Access | Payload | Result |
| ----------- | -------- | --------- | -------- |
| `auth.sign_in` | Public | `username`, `password` | `authenticated` |
| `auth.sign_out` | Administrator | `{}` | `authenticated: false` |
| `bookmark.list` | Public | `page`, optional `tag` | paginated bookmark summary |
| `bookmark.search` | Public | optional `tag`, optional `words`, `page` | capped paginated bookmark summary |
| `capture.start` | Administrator | `url`, optional `bookmark_id` | capture draft status |
| `capture.status` | Draft owner | `capture_id` | capture draft status |
| `capture.retry` | Draft owner | `capture_id` | capture draft status |
| `capture.discard` | Draft owner | `capture_id` | terminal discarded status |
| `bookmark.create` | Administrator | `url`, `description`, `tags`, optional `capture_id`, `save_without_screenshot` | created bookmark |
| `bookmark.update` | Administrator | `bookmark_id`, `url`, `description`, `tags`, optional `capture_id`, `save_without_screenshot` | updated bookmark |
| `bookmark.delete` | Administrator | `bookmark_id`, `confirm: true` | deleted identifier and cleanup status |
| `configuration.get` | Administrator | `{}` | redacted editable configuration |
| `configuration.validate` | Administrator | allowed configuration candidate | normalized candidate and restart requirement; no activation |
| `configuration.update` | Administrator | allowed configuration candidate | activated redacted configuration and restart requirement |
| `system.migrate` | Administrator or deployment-authorized startup mode | `{}` | starting and resulting schema versions |

`system.migrate` is disabled unless deployment configuration explicitly permits migrations for the CLI process. It never accepts arbitrary SQL or migration paths.

## Shared Value Contracts

### Bookmark Summary

```json
{
  "id": "018f...",
  "url": "https://example.test/reference",
  "description": "Database transaction reference",
  "created_at": "2026-08-31T12:00:00Z",
  "updated_at": "2026-08-31T12:00:00Z",
  "tags": [{"name": "Go"}],
  "screenshot": {"id": "018f...", "available": true}
}
```

The full URL is part of requested bookmark data and may appear on stdout; it remains excluded from diagnostics. A missing screenshot returns `available: false` without failing the bookmark operation.

### Pagination

```json
{"page":2,"page_size":10,"last_page":7,"total_within_cap":67,"is_capped":false,"items":[]}
```

List fixes `page_size` at 10. Search uses configuration. Page values below 1 or beyond the final page return the nearest valid page. An empty collection returns page 1, last page 1, and no items.

### Search

At least one non-empty `tag` or `words` value is required. Tag matching is exact after normalization. Description words use language-neutral case-insensitive all-word matching in any order. Supplying both combines them with AND. `is_capped` reports that the configured maximum truncated further matches.

### Capture Draft Status

```json
{"capture_id":"018f...","state":"ready","can_retry":false,"can_continue_without":true,"failure_code":null}
```

The response never includes absolute filesystem paths, `agent-browser` command arguments, raw process output, or a sensitive final URL. Capture is bounded by the same timeout and concurrency policy as HTTP. A non-HTTP/HTTPS URL returns `invalid_url` before process execution; public and private destinations are otherwise permitted.

### Bookmark Write

- `tags` is an array of strings. Empty entries are ignored and normalized duplicates collapse.
- `capture_id`, when present, must be ready, unexpired, stream-session owned, URL-matching, and bookmark-matching for an update.
- `save_without_screenshot` must be true when no usable capture exists after a failed/discarded attempt.
- Duplicate bookmark URLs are valid.
- Update retains original `created_at`.
- Delete requires literal JSON boolean `confirm: true`; omission or false returns `confirmation_required` without mutation.

### Redacted Configuration

Configuration payloads may include branding, default view, screenshot root, non-secret database fields, the database password environment-variable **name**, and search limits. They must not include administrator username, plaintext password, password hash, a resolved environment value, session data, or CSRF data. Any such field is rejected as `unknown_field` rather than ignored.

## Error Codes

| Code | Meaning |
| ------ | --------- |
| `invalid_json` | Line is not one strict JSON request object. |
| `unsupported_version` | `version` is not `v1`. |
| `unknown_operation` | Operation is not registered. |
| `unknown_field` | Envelope or payload contains a non-contract field. |
| `validation_failed` | One or more typed fields are invalid. |
| `authentication_required` | Protected operation has no valid stream session. |
| `authentication_failed` | Generic credential failure without identifying cause. |
| `login_blocked` | Pair is temporarily blocked; includes safe retry delay. |
| `forbidden` | Authenticated stream does not own the requested draft/resource. |
| `not_found` | Resource is absent or intentionally not distinguished. |
| `conflict` | Draft is stale, consumed, expired, or in an invalid state. |
| `capture_failed` | Capture reached a terminal safe failure; retry may be allowed. |
| `capacity_limited` | Capture concurrency is full; includes safe retry delay. |
| `dependency_unavailable` | PostgreSQL, filesystem, or executable is unavailable. |
| `internal_error` | Unexpected failure; stderr correlation uses request `id`. |

## Compatibility

Additive optional result fields may appear within protocol `v1`; clients must ignore unknown **response** fields. Request fields remain strict to catch mistakes. Removing or changing the meaning/type of an operation, request field, response field, or error code requires a new protocol version and migration guide. The executable and importable library follow the semantic-version policy in [plan.md](../plan.md).
