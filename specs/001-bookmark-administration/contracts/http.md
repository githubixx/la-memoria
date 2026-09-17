# HTTP and htmx Contract

**Contract version**: `v1`  
**Feature**: [Bookmarker Administration](../spec.md)  
**Model**: [data-model.md](../data-model.md)

## Protocol Conventions

- All application responses use UTF-8.
- Browser forms use `application/x-www-form-urlencoded` or `multipart/form-data` only where a validated file field is introduced.
- GET and HEAD are safe and have no state-changing side effects. State changes use POST so they work with ordinary HTML forms as well as htmx.
- Each response carries or creates a non-secret `X-Request-ID`. A valid incoming request ID may be retained; otherwise Bookmarker replaces it.
- Public pages may be cached only when their response does not vary by authenticated controls. Protected pages, login responses, validation errors, and responses containing CSRF tokens use `Cache-Control: no-store`.
- External bookmark links use `target="_blank"` and `rel="noopener noreferrer"`.
- Dates render in the configured deployment locale/time zone while form and persistence values remain unambiguous UTC instants.

## Full-Page and Fragment Negotiation

Every GET route used by htmx can be loaded directly. With no `HX-Request` header, the handler returns a complete HTML document including layout, title, menu, main content, and local assets. With `HX-Request: true`, it returns the route's main-content fragment or the narrow fragment documented by the triggering control.

All negotiable responses include:

```http
Vary: HX-Request
```

An htmx success that changes the canonical location returns `HX-Redirect` to a GET route. The equivalent non-htmx response is `303 See Other` with `Location`. Validation responses return status 422 and replace the submitted form while preserving all valid non-secret values. Protected htmx requests whose session is absent or invalid return `401 Unauthorized` with `HX-Redirect: /login?return_to=...`; direct protected GET requests use a 303 redirect to that local login URL.

## Security Contract

### Browser Session

Bookmarker sends one opaque session cookie:

```http
Set-Cookie: bookmarker_session=<random-token>; Path=/; Secure; HttpOnly; SameSite=Lax
```

The cookie has no `Expires` or `Max-Age`. Sign-in rotates both session and CSRF tokens. Sign-out revokes server state before clearing the cookie. An absent, unknown, or revoked token has public-only privileges.

### CSRF and Origin

Every POST requires both:

1. A permitted same-origin `Origin` value, or a valid same-site `Referer` fallback when `Origin` is absent.
2. A cryptographically random synchronizer value in form field `csrf_token` matching the current server-side session digest.

Anonymous GET `/login` establishes a short-lived anonymous session and login token. Authentication rotates that session. htmx sends the same rendered form token as an ordinary form. Missing or invalid origin/token evidence returns `403 Forbidden`, performs no mutation, and does not reveal which check failed.

Forwarded source addresses are honored only when the immediate peer belongs to a configured trusted-proxy CIDR. Otherwise the peer address is authoritative for login throttling.

### Authentication Failure

Invalid username, invalid password, and an active pair-specific block return the same user-visible authentication error. The fifth failure in 10 minutes blocks that normalized username/source-address pair for 15 minutes. Blocked responses return `429 Too Many Requests` and a bounded `Retry-After`; other failures return 422. Passwords, hashes, source-pair digests, cookies, and CSRF values never appear in responses or logs.

## Routes

| Method | Path | Access | Success | Purpose |
| -------- | ------ | -------- | --------- | --------- |
| GET | `/` | Public | 303 | Redirect to configured default; a protected default passes through login. |
| GET | `/bookmarks` | Public | 200 | Newest-first List view, optional exact tag filter, fixed 10-item pages. |
| GET | `/search` | Public | 200 | Search form or paginated results for tag and/or description words. |
| GET | `/login` | Public | 200 | Sign-in form and anonymous CSRF session. |
| POST | `/login` | Public + CSRF | 303 or `HX-Redirect` | Authenticate and rotate into an administrator session. |
| POST | `/logout` | Administrator + CSRF | 303 or `HX-Redirect` | Revoke the session and return to List. |
| GET | `/bookmarks/new` | Administrator | 200 | Add form. |
| POST | `/captures` | Administrator + CSRF | 201 | Start and report a session-owned screenshot draft for a form URL. |
| GET | `/captures/{capture_id}` | Draft owner | 200 | Return current capture status/preview as a page or fragment. |
| POST | `/captures/{capture_id}/retry` | Draft owner + CSRF | 200 | Retry a failed, unexpired draft. |
| POST | `/captures/{capture_id}/discard` | Draft owner + CSRF | 200 | Discard staged output and mark continuation without a screenshot. |
| POST | `/bookmarks` | Administrator + CSRF | 303 or `HX-Redirect` | Create a bookmark, tags, and optional consumed screenshot. |
| GET | `/bookmarks/{bookmark_id}/edit` | Administrator | 200 | Edit form with current fields and screenshot state. |
| POST | `/bookmarks/{bookmark_id}` | Administrator + CSRF | 303 or `HX-Redirect` | Save URL, description, tags, and optional replacement screenshot. |
| GET | `/bookmarks/{bookmark_id}/delete` | Administrator | 200 | Render explicit delete confirmation; no deletion occurs. |
| POST | `/bookmarks/{bookmark_id}/delete` | Administrator + CSRF | 303 or `HX-Redirect` | Confirm compensated bookmark/screenshot deletion. |
| GET | `/configuration` | Administrator | 200 | Render editable redacted settings; credentials are absent. |
| POST | `/configuration` | Administrator + CSRF | 303 or `HX-Redirect` | Validate and atomically activate an allowed configuration candidate. |
| GET | `/screenshots/{screenshot_id}` | Public | 200 or 404 | Serve a validated promoted image; the UI substitutes its placeholder on 404. |
| GET | `/favicon` | Public | 200 | Serve the validated configured favicon or built-in default. |
| GET | `/assets/{asset_path}` | Public | 200 | Serve fingerprinted local CSS, minimal JavaScript, and vendored htmx. |

Opaque path identifiers that are malformed return 404, not parser details. Protected resource lookup applies authorization before distinguishing missing resources.

## List Contract

### Request

```http
GET /bookmarks?page=3&tag=distributed%20systems
```

| Parameter | Required | Rules                                                                                    |
|-----------|----------|-------                                                                                   |
| `page`    | No       | Positive decimal integer; defaults to 1 and clamps to the nearest available page.        |
| `tag`     | No       | Trimmed and normalized for exact case-insensitive tag comparison; blank means no filter. |

The view renders URL, description, creation date, and tags in that order. Signed-in responses additionally render Edit and Delete controls. Pagination is absent for zero/one page and otherwise includes Previous, bounded page links, current-page state, ellipsis where omitted, final page, and Next.

## Search Contract

### Request

```http
GET /search?tag=go&q=database+transactions&page=2
```

| Parameter | Required | Rules |
| ----------- | ---------- | ------- |
| `tag` | Conditional | Exact normalized tag; at least `tag` or `q` must be non-empty to execute a search. |
| `q` | Conditional | Plain description words; punctuation is safely parsed and all resulting words must match case-insensitively in any order. |
| `page` | No | Positive decimal integer clamped to an available page. |

When both criteria are supplied they combine with AND. Results use configured page size and maximum, remain newest first, and visibly report when the maximum capped additional matches. A form loaded without criteria is valid; an explicitly submitted empty search returns 422 with guidance rather than all bookmarks.

## Bookmark Form Contract

| Field | Required | Rules |
| ------- | ---------- | ------- |
| `url` | Yes | Absolute HTTP/HTTPS URL without embedded credentials. Public and private destinations are accepted for capture. |
| `description` | Yes | Non-empty trimmed plain text. |
| `tags` | No | Repeated values. Empty values are ignored and normalized duplicates collapse to one. |
| `capture_id` | Conditional | Opaque ready draft owned by this session, matching URL and optional edited bookmark. |
| `save_without_screenshot` | Conditional | Must be `true` to continue after capture failure/discard when no usable draft exists. |
| `csrf_token` | Yes | Current synchronizer token. |

Changing a URL invalidates any prior `capture_id` in the rendered form and offers a replacement capture. A ready replacement is not promoted until bookmark save succeeds. Edit preserves `created_at`. Successful delete requires the POST confirmation and removes the record from browse/search immediately even when post-commit file cleanup needs an operator-visible retry.

## Capture Contract

POST `/captures` accepts `url`, optional `bookmark_id`, and `csrf_token`. It rejects non-HTTP/HTTPS URLs with 422. The response model is:

| Field | Meaning |
| ------- | --------- |
| `capture_id` | Opaque draft identifier placed in the bookmark form. |
| `state` | `pending`, `capturing`, `ready`, or `failed` for an active response. |
| `preview_url` | Present only when ready; session ownership is checked when draft state is not public. |
| `failure_code` | Stable safe code when failed. |
| `message` | Concise user-safe progress or failure text. |
| `can_retry` | Whether POST retry is currently valid. |
| `can_continue_without` | Whether the form may explicitly omit a screenshot. |

Capture follows ordinary redirects and validates that the final destination is HTTP/HTTPS. Timeout returns a failed draft within the bounded request budget; it does not leave a partial bookmark.

## Configuration Form Contract

The GET and validation response may contain only:

- `page_title`
- `favicon_path` or a non-secret favicon upload/reference field selected by implementation
- `default_view`
- `screenshot_root`
- database host, port, database name, user, password environment-variable **name**, and TLS mode
- `search_page_size`
- `maximum_search_results`
- restart-required status for settings that cannot activate in-process

Administrator username, password, password hash, resolved database password, session values, and environment values are never fields, placeholders, data attributes, comments, or serialized htmx state. Invalid candidates return 422 with field errors and preserve the previous configuration. Connectivity/path checks may return 503 when a dependency cannot be validated, still without activation. A successful change is durable before its redirect and indicates whether restart is required.

## Error Contract

| Status | Meaning |
| -------- | --------- |
| 400 | Malformed syntax or unsupported request encoding. |
| 401 | Missing/invalid administrator session on an htmx or unsafe request. |
| 403 | Valid session lacks CSRF/origin evidence or resource ownership. |
| 404 | Route/resource is absent or intentionally not distinguished. |
| 409 | Stale/consumed capture draft or conflicting state transition. |
| 413 | Request or generated asset exceeds configured bound. |
| 422 | Field validation or ordinary authentication failure. |
| 429 | Active username/source-address pair block or bounded capture concurrency limit. |
| 500 | Unexpected internal failure with request ID and no sensitive detail. |
| 503 | Database, filesystem, capture executable, or configuration validation dependency unavailable. |

HTML error responses retain the global menu where safe, identify an actionable next step, and include the request ID for operational correlation. They never expose SQL, paths outside configured display fields, command arguments, environment values, stack traces, or secret material.
