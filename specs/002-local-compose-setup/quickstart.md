# Quickstart: Validate Local Docker Compose Setup

## Prerequisites

- Docker-compatible container tooling with the Docker Compose plugin.
- Network access during the first image build to obtain declared build dependencies and the pinned browser runtime.
- An available local TCP port `8080`, or a chosen replacement value.

## 1. Supply Local Secrets

From the repository root, generate a verifier. The bootstrap-only hash service builds the Bookmarker image if needed and prompts for the password without echoing it; it does not need an environment file:

```sh
docker compose -f deploy/compose/docker-compose.yml run --rm bookmarker-hash
```

Copy `deploy/compose/.env.example` to `deploy/compose/.env`, place the printed verifier in `BOOKMARKER_ADMIN_PASSWORD_HASH`, choose a non-empty `BOOKMARKER_DB_PASSWORD`, and set a non-empty `BOOKMARKER_ADMIN_USERNAME`. The username must use only letters, digits, `.`, `_`, and `-`. Do not add the plaintext administrator password to any file.

See [local-compose.md](contracts/local-compose.md) for the environment-input contract and [data-model.md](data-model.md) for credential handling.

## 2. Start and Verify Readiness

```sh
docker compose -f deploy/compose/docker-compose.yml up --build --detach
docker compose -f deploy/compose/docker-compose.yml ps
```

Expected result: PostgreSQL reports healthy, Bookmarker becomes healthy after it connects and runs migrations, and the application opens at `http://127.0.0.1:8080` (or the configured `BOOKMARKER_HOST_PORT`).

Sign in with the configured administrator username and the plaintext password used to create the verifier. Add a bookmark with a screenshot and record a unique description for the restart check.

## 3. Verify Persistence

```sh
docker compose -f deploy/compose/docker-compose.yml down
docker compose -f deploy/compose/docker-compose.yml up --detach
docker compose -f deploy/compose/docker-compose.yml ps
```

Expected result: after both services are healthy, the administrator can sign in and find the bookmark and its screenshot. This verifies the PostgreSQL and screenshot persistent stores described in [data-model.md](data-model.md).

## 4. Verify Template Adaptation

1. Copy the complete `deploy/compose/` directory into a separate Bookmarker source checkout.
2. Supply the three required environment inputs in that bundle's `.env` file.
3. Set `BOOKMARKER_HOST_PORT` to an unused port, for example `18080`, and optionally set `BOOKMARKER_POSTGRES_VOLUME` and `BOOKMARKER_SCREENSHOTS_VOLUME` to deployment-specific named-volume names.
4. Start the copied bundle and open `http://127.0.0.1:18080`.

Expected result: Bookmarker is reachable on the replacement host port, still connects to PostgreSQL internally, and retains its data in the chosen named volumes. Bind-mount storage is outside this template's supported configuration surface.

## 5. Validate Expected Failures

- Omit a required environment input and run `docker compose -f deploy/compose/docker-compose.yml up`; expect a non-zero startup with a non-secret explanation of the missing setting.
- Supply a malformed `BOOKMARKER_ADMIN_PASSWORD_HASH`; expect Bookmarker configuration validation to reject it before serving traffic.
- Occupy the configured host port; expect Compose to report the port conflict. Change only `BOOKMARKER_HOST_PORT`, then start again.
- Prevent PostgreSQL from becoming healthy; expect Bookmarker not to start and Compose status/logs to identify the unavailable dependency without exposing secrets.

## Cleanup

```sh
docker compose -f deploy/compose/docker-compose.yml down
```

This retains local bookmarks and screenshots. Use `docker compose -f deploy/compose/docker-compose.yml down --volumes` only when intentionally deleting all local Bookmarker data.
