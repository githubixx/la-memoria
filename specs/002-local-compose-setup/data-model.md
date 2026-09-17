# Data Model: Local Docker Compose Setup

## Local Compose Deployment Bundle

Represents the committed, copyable `deploy/compose/` directory that operates within a Bookmarker source checkout.

| Field | Description | Validation / default |
| --- | --- | --- |
| `bookmarker` service | Application image, runtime configuration renderer, and web process. | Built from the containing source checkout. |
| `postgres` service | Private PostgreSQL dependency. | Reachable only through the Compose network. |
| `bookmarker-hash` service | Bootstrap-only password verifier generator. | Uses the Bookmarker image without normal-service environment or PostgreSQL dependencies. |
| `BOOKMARKER_HOST_PORT` | Host-side port mapped to Bookmarker. | Defaults to `8080`; users may change it for a conflict. |
| `BOOKMARKER_POSTGRES_VOLUME` | PostgreSQL named-volume name. | Defaults to `bookmarker-postgres-data`; retained unless explicitly removed. |
| `BOOKMARKER_SCREENSHOTS_VOLUME` | Screenshot named-volume name. | Defaults to `bookmarker-screenshots`; mounted writable at Bookmarker's configured screenshot root. |

**Relationships**:

- The `bookmarker` service depends on PostgreSQL health and connects to the `postgres` service by its internal service name.
- The `bookmarker-hash` service shares the Bookmarker image but has no dependency on either normal service.
- The database volume belongs to PostgreSQL; the screenshot volume belongs to Bookmarker.
- The deployment bundle contains all deployment artifacts and builds from its containing Bookmarker source checkout.

## Local Environment Settings

Represents the minimum user-provided runtime settings, typically in an ignored local environment file or shell environment.

| Field | Description | Validation / default |
| --- | --- | --- |
| `BOOKMARKER_ADMIN_USERNAME` | Administrator identity written to runtime YAML. | Required, non-empty. |
| `BOOKMARKER_ADMIN_PASSWORD_HASH` | Argon2id PHC verifier written to runtime YAML. | Required; must pass Bookmarker PHC validation. |
| `BOOKMARKER_DB_PASSWORD` | Database credential supplied to Bookmarker and PostgreSQL at runtime. | Required, non-empty; never written to YAML or logged. |
| `BOOKMARKER_HOST_PORT` | Optional published web port. | Defaults to `8080`; must be an available host port. |
| `BOOKMARKER_POSTGRES_VOLUME` | Optional PostgreSQL named-volume override. | Must be a valid Docker volume name. |
| `BOOKMARKER_SCREENSHOTS_VOLUME` | Optional screenshot named-volume override. | Must be a valid Docker volume name. |

**State transitions**:

1. Absent: local startup is rejected with an actionable error.
2. Supplied: entrypoint renders owner-only runtime YAML.
3. Validated: Bookmarker accepts the generated configuration and starts after PostgreSQL is healthy.
4. Invalid: Bookmarker exits before serving requests without printing a secret.

## Runtime Configuration

Represents the generated, container-local `config.yaml` consumed by Bookmarker.

| Field | Source | Handling |
| --- | --- | --- |
| `administrator.username` | `BOOKMARKER_ADMIN_USERNAME` | Required; not exposed by interfaces. |
| `administrator.password_hash` | `BOOKMARKER_ADMIN_PASSWORD_HASH` | Required salted verifier; mode `0600`; not logged. |
| `database.host`, `database.port`, `database.name`, `database.user` | Compose defaults | Use the private `postgres` service and matching defaults. |
| `database.password_env` | Fixed configuration reference | Remains `BOOKMARKER_DB_PASSWORD`; actual password stays in process environment. |
| `database.tls_mode` | Local default | `disable` for the private local network only. |
| `screenshots.root` | Compose default | Writable path backed by the screenshots volume. |
| `server.listen_address` | Compose default | Container-facing `0.0.0.0:8080`. |

`BOOKMARKER_ADMIN_USERNAME` accepts only non-empty single-line identifier characters (`A-Z`, `a-z`, `0-9`, `.`, `_`, `-`). `BOOKMARKER_ADMIN_PASSWORD_HASH` must be a single-line Argon2id PHC verifier. The entrypoint rejects other values before YAML rendering.

## Administrator Password Verifier

Represents a non-plaintext password credential generated before the stack starts.

| Field | Description | Validation |
| --- | --- | --- |
| algorithm | Password hashing algorithm. | `argon2id`. |
| version | PHC version. | `v=19`. |
| parameters | Memory, time, and parallelism cost. | `m=65536,t=3,p=1`. |
| salt | Per-verifier random salt. | Raw base64 encoding, fresh for every generation. |
| derived key | Password-derived key. | Raw base64 encoding, compatible with verifier. |

**State transitions**:

1. Password is entered interactively and never serialized by the command.
2. Generator creates random salt and prints a one-line verifier to stdout.
3. User places the verifier in their ignored runtime environment input.
4. Runtime configuration carries only the verifier; Bookmarker validates it before startup.

## Persistent Local Data

Existing application entities retained across service recreation.

| Entity | Store | Relationship |
| --- | --- | --- |
| Bookmark records and related tables | PostgreSQL named volume | Restored when the PostgreSQL service restarts. |
| Captured screenshots | Screenshot named volume | Resolved by stored screenshot references in the database. |

The stack must preserve both stores together; removing either volume is an explicit destructive reset.
