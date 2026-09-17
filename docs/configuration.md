# Configuration

Bookmarker loads one strict YAML document from `BOOKMARKER_CONFIG`, or
`config.yaml` in the working directory. Production configuration must have
mode `0600`; group- or world-readable files are rejected at startup and before
an update is activated.

| Field | Purpose |
| --- | --- |
| `branding.page_title` | Non-empty title, up to 120 characters. |
| `branding.favicon_path` | Optional PNG below `web/static`. |
| `default_view` | `list`, `add`, `search`, or `configuration`. |
| `screenshots.root` | Writable screenshot staging and promoted-file root. |
| `database.*` | Connection host, port, name, user, TLS mode, and password environment-variable name. |
| `search.page_size` | Integer from 1 through 100. |
| `search.maximum_results` | Integer at least the page size and no greater than 10000. |

The web and CLI configuration interfaces redact the administrator identity,
password verifier, resolved database password, tokens, and all environment
values. Database connection changes require an application restart after their
validated atomic activation. The YAML adapter writes a same-directory temporary
file with mode `0600`, syncs it, renames it atomically, and syncs the parent
directory before success is reported.

## Local Compose Configuration

The local Docker Compose bundle is under `deploy/compose/`. Its ignored `.env`
file supplies `BOOKMARKER_ADMIN_USERNAME`, `BOOKMARKER_ADMIN_PASSWORD_HASH`,
and `BOOKMARKER_DB_PASSWORD`. The container entrypoint validates the first two
values before rendering an owner-only runtime YAML file; the database password
remains in the process environment and is never written to YAML.

Use the bundled `bookmarker-hash` Compose service to generate a compatible
Argon2id verifier before creating `.env`. See the local Docker Compose section
in the [README](../README.md) for startup, persistence, and supported host-port
and named-volume overrides.
