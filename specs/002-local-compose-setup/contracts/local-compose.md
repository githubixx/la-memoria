# Contract: Local Docker Compose Setup

## Purpose

Defines the user-facing Compose and password-hash interfaces. See [data-model.md](../data-model.md) for field definitions.

## Compose Interface

### Services

| Service | Exposed interface | Required behavior |
| --- | --- | --- |
| `bookmarker` | `${BOOKMARKER_HOST_PORT:-8080}:8080` | Serves Bookmarker only after configuration validation, database connection, migration, and initialization succeed. |
| `postgres` | Compose-private `postgres:5432` | Persists application data and signals healthy readiness before Bookmarker starts. No host port is published by default. |
| `bookmarker-hash` | No network or published port | Runs the verifier generator before normal configuration exists, without PostgreSQL or required normal-service environment inputs. |

### Required Environment Inputs

| Name | Consumer | Required behavior |
| --- | --- | --- |
| `BOOKMARKER_ADMIN_USERNAME` | configuration renderer | Must be non-empty. |
| `BOOKMARKER_ADMIN_PASSWORD_HASH` | configuration renderer | Must be a valid Argon2id PHC verifier. |
| `BOOKMARKER_DB_PASSWORD` | PostgreSQL and Bookmarker | Must be non-empty and is never serialized into configuration or output. |

### Optional Environment Inputs

| Name | Default | Behavior |
| --- | --- | --- |
| `BOOKMARKER_HOST_PORT` | `8080` | Changes only the host-side browser port. Internal Bookmarker-to-PostgreSQL connectivity remains unchanged. |
| `BOOKMARKER_POSTGRES_VOLUME` | `bookmarker-postgres-data` | Changes the named volume retaining PostgreSQL data. |
| `BOOKMARKER_SCREENSHOTS_VOLUME` | `bookmarker-screenshots` | Changes the named volume retaining captured screenshots. |

### Lifecycle Commands

| Command | Contract |
| --- | --- |
| `docker compose up --build` | Builds the Bookmarker image and starts the two services using user-provided required settings. |
| `docker compose down` | Stops services while retaining named volumes. |
| `docker compose down --volumes` | Explicitly deletes bookmark and screenshot data. |
| `docker compose ps` | Shows health and service status for troubleshooting. |

## Password Hash CLI Interface

### Invocation

```sh
docker compose -f deploy/compose/docker-compose.yml run --rm bookmarker-hash
```

The command must work before the normal stack is configured and before an environment file exists. It prompts for the password through the terminal without echoing it when interactive.

## Deployment Bundle Contract

- The supported copy unit is the complete `deploy/compose/` directory inside a Bookmarker source checkout.
- The bundle contains its Dockerfile, Compose file, runtime configuration template, entrypoint, and placeholder-only environment example.
- The Compose build context is the containing source checkout. Standalone copied deployment without Bookmarker source or a published Bookmarker image is outside this feature's support boundary.

### Output Contract

- Standard output contains exactly one newline-terminated verifier matching `$argon2id$v=19$m=65536,t=3,p=1$<salt>$<hash>`.
- Standard error contains prompts and diagnostics only.
- The plaintext password is never written to standard output, standard error, the generated YAML, or logs.
- Failure to obtain a non-empty password exits non-zero and emits an actionable non-secret diagnostic on standard error.

## Runtime Configuration Rendering Contract

1. The entrypoint verifies that all required environment inputs exist and are non-empty without echoing their values.
2. It permits only a single-line administrator username containing `A-Z`, `a-z`, `0-9`, `.`, `_`, and `-`, and a single-line Argon2id PHC verifier before rendering either value into YAML.
3. It renders `config.yaml` into a container-private runtime directory with mode `0600`.
4. It configures the internal PostgreSQL hostname, screenshot volume path, unencrypted local database connection, and container listen address.
5. It executes `bookmarker-web` for the normal service.
6. An invalid verifier or unavailable dependency causes the relevant process to exit non-zero before Bookmarker is available, with a non-secret actionable diagnostic.

## Readiness and Persistence Contract

- PostgreSQL health is established before Bookmarker starts.
- The Bookmarker health check uses a successful public HTTP response from the running service.
- Bookmarker runs migrations before accepting traffic.
- Normal stop/start and `docker compose down`/`up` cycles retain the PostgreSQL and screenshots named volumes.
