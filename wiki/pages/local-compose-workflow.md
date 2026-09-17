---
title: Local Compose workflow
type: howto
sources: [S009, S010]
updated: 2026-09-02
---

# Local Compose workflow

Use the bootstrap-only `bookmarker-hash` Compose service to generate an administrator Argon2id verifier before creating the normal environment file; the service has no normal-service secret or database requirement. (S009) (S010)

Copy `.env.example` to `.env`, then supply a username, the generated verifier, and a non-empty database password. Plaintext administrator passwords must not be placed in configuration, and the generated verifier should not be shared unnecessarily. (S009) (S010)

Start the stack with `docker compose -f deploy/compose/docker-compose.yml up --build --detach`, verify service health with `docker compose -f deploy/compose/docker-compose.yml ps`, and browse to `http://127.0.0.1:8080` unless `BOOKMARKER_HOST_PORT` changes the host port. (S009) (S010)

For a persistence check, stop with `down`, start again with `up --detach`, and confirm the bookmark and screenshot remain available; use `down --volumes` only to intentionally delete the named data stores. (S009)

This bundle is for local evaluation. Review credentials, network exposure, backups, and transport security before adapting it for non-local use. (S009)

The underlying service topology and its configuration safeguards are documented in [`local Compose deployment`](./local-compose-deployment.md). (S009)
