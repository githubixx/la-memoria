# la-memoria

Bookmarker is a Go library and server-rendered web/JSON Lines application for
public bookmark browsing and authenticated administration.

At the current state it's not really usable. It was basically a PoC for getting
started with [Github's Spec Kit](https://github.com/github/spec-kit) and
[Spec-Driven Development (SDD)](https://github.com/github/spec-kit#-what-is-spec-driven-development).

## Setup

Install Go 1.27, Docker-compatible container tooling, and Node.js LTS for
[agent-browser](https://github.com/vercel-labs/agent-browser). Then install dependencies:

```sh
go mod download
npm ci
npx playwright install chromium firefox
```

Create the deployment configuration from `config.example.yaml`, set owner-only
permissions, and provide the referenced database password through the
environment:

```sh
cp config.example.yaml config.yaml
chmod 0600 config.yaml
export BOOKMARKER_DB_PASSWORD='replace-this-value'
export BOOKMARKER_CONFIG="$PWD/config.yaml"
```

`config.yaml` is intentionally ignored by Git. Startup rejects a configuration
file with any group or other permission bit. Keep the administrator password
verifier in deployment configuration, never plaintext. Generate a verifier by
running the following command, which prompts for the administrator password
without echoing it and prints a salted Argon2id PHC verifier:

```sh
go run ./cmd/bookmarker-hash
```

Set `administrator.password_hash` in `config.yaml` to the complete printed
verifier. To rotate the administrator password, generate a new verifier,
replace that value, and restart the application. Database passwords remain
environment values and must never be written to YAML.

Apply schema migrations through the deployment-authorized CLI process:

```sh
BOOKMARKER_ALLOW_MIGRATIONS=true \
  printf '%s\n' '{"version":"v1","id":"migrate-1","operation":"system.migrate","payload":{}}' \
  | go run ./cmd/bookmarker-cli
```

Run the web application with `go run ./cmd/bookmarker-web`. The CLI reads
versioned JSON Lines from stdin and emits exactly one JSON response per input
line on stdout; diagnostics are written only to stderr.

Back up PostgreSQL with standard `pg_dump` procedures and the configured
screenshot root together. Restore the database first, then restore screenshot
files under the configured root with their relative keys intact.

## Local Docker Compose

For a self-contained local evaluation environment, install Docker with the
Docker Compose plugin. The bundle at `deploy/compose/` builds from this source
checkout and starts Bookmarker with a private PostgreSQL service.

First generate an administrator verifier. The command prompts without echoing
the plaintext password and prints only the salted Argon2id verifier:

```sh
docker compose -f deploy/compose/docker-compose.yml run --rm bookmarker-hash
```

Copy the non-secret example, then set `BOOKMARKER_ADMIN_USERNAME`, the printed
`BOOKMARKER_ADMIN_PASSWORD_HASH`, and a chosen non-empty
`BOOKMARKER_DB_PASSWORD`. Do not put the plaintext administrator password in
the file or share the verifier unnecessarily.

```sh
cp deploy/compose/.env.example deploy/compose/.env
docker compose -f deploy/compose/docker-compose.yml up --build --detach
```

Open `http://127.0.0.1:8080` and sign in with the chosen administrator
username and plaintext password.

```sh
docker compose -f deploy/compose/docker-compose.yml ps
```

shows PostgreSQL and Bookmarker health. Stop the environment with

```sh
docker compose -f deploy/compose/docker-compose.yml down
```

Named volumes keep bookmarks and screenshots until `down --volumes` is used.

`BOOKMARKER_HOST_PORT` changes only the browser port. Set
`BOOKMARKER_POSTGRES_VOLUME` and `BOOKMARKER_SCREENSHOTS_VOLUME` to choose
deployment-specific named-volume names. PostgreSQL is intentionally not
published to the host. This bundle is for local evaluation: review credentials,
network exposure, backups, and transport security before non-local use.

## Validation

```sh
go test ./...
go test -tags=contract ./tests/contract/...
go test -tags=integration ./tests/integration/...
go vet ./...
npx playwright test
```
