---
title: Local Compose deployment
type: component
sources: [S009, S010]
updated: 2026-09-02
---

# Local Compose deployment

The copyable `deploy/compose/` bundle builds Bookmarker from its containing source checkout and packages the web binary, password-hash binary, migrations, and web assets in a runtime image. (S009) (S010)

The Compose stack has three services: private PostgreSQL, the Bookmarker web service, and a bootstrap-only `bookmarker-hash` service. PostgreSQL has no host port, while the web service maps `${BOOKMARKER_HOST_PORT:-8080}` to container port 8080. (S009) (S010)

PostgreSQL data and screenshots use separate named volumes whose names can be overridden with `BOOKMARKER_POSTGRES_VOLUME` and `BOOKMARKER_SCREENSHOTS_VOLUME`; ordinary `docker compose down` retains both stores. (S009) (S010)

The web service waits for PostgreSQL to report healthy, and its own health check requests `/bookmarks`; this makes Compose health represent a serving application rather than only a listening database port. (S009) (S010)

At startup, the entrypoint requires a non-empty administrator username, Argon2id verifier, and database password; it restricts the YAML-rendered username and verifier formats, writes the generated configuration with mode `0600`, and does not render the database password into YAML. (S009) (S010)

The [`local Compose workflow`](./local-compose-workflow.md) describes the operator-facing bootstrap, startup, persistence, and cleanup steps. (S009)
