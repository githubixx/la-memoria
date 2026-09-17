# Implementation Plan: Local Docker Compose Setup

**Branch**: `002-local-compose-setup` | **Date**: 2026-09-02 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from [spec.md](spec.md)

**Note**: This template is filled in by the `/speckit.plan` command; its definition describes the execution workflow.

## Summary

Provide a reproducible Docker Compose local-evaluation environment through a complete `deploy/compose/` bundle built from a Bookmarker source checkout. It contains a private PostgreSQL service, a normal Bookmarker service with persistent named volumes, and a bootstrap-only `bookmarker-hash` service that runs without normal startup secrets. A container entrypoint restricts YAML-rendered inputs to a safe grammar and creates the required `0600` runtime YAML from user-provided environment inputs; README guidance and a committed non-secret environment example make the bundle copyable between source checkouts without committed runnable credentials.

## Technical Context

**Language/Version**: Go 1.27.0; POSIX shell for the image entrypoint; Docker Compose YAML

**Primary Dependencies**: Go standard library and `golang.org/x/crypto/argon2`; Docker-compatible container tooling with Docker Compose; PostgreSQL image; `agent-browser` 0.35.2 and its browser runtime

**Storage**: Configurable named volume names for Bookmarker's PostgreSQL data and `/var/lib/bookmarker/screenshots`; ephemeral owner-only generated runtime YAML

**Testing**: Go unit tests (`go test ./bookmarker/...`); Go contract tests (`go test -tags=contract ./tests/contract/...`); opt-in Compose integration test using Docker; existing Playwright E2E suite for application flow

**Target Platform**: Linux containers running on a Docker-compatible local host; modern browser accessing `http://127.0.0.1:${BOOKMARKER_HOST_PORT:-8080}`

**Project Type**: Go library with JSON Lines CLI and server-rendered web application; deployment template and documentation

**Performance Goals**: A healthy local stack is available after image build and service initialization; page and capture behavior retains the existing feature targets

**Constraints**: No plaintext administrator password or database password is committed, rendered into YAML, or logged; the bootstrap hash service works without `.env`; YAML-rendered values use a constrained single-line grammar; generated config must be `0600`; PostgreSQL has no host-port exposure by default; migrations complete before service readiness; unavailable PostgreSQL has actionable non-secret failure evidence; the pinned `agent-browser` version must be available on `PATH`

**Scale/Scope**: One copyable `deploy/compose/` source-checkout bundle, one multi-stage image definition, a non-secret environment example, runtime config template/entrypoint, a bootstrap hash service and focused password-hash CLI, README onboarding, and focused unit/contract/integration coverage

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-checked after Phase 1 design: PASS.*

| Principle / gate | Plan response | Status |
| --- | --- | --- |
| I. Library-first architecture | Add password generation to the existing `bookmarker/usecase` library and expose it through a dedicated CLI. Compose/config rendering remains deployment wiring, not application behavior. | PASS |
| II. Composable command-line interfaces | The bootstrap-only `bookmarker-hash` service writes the one-line machine-consumable verifier to stdout and sends prompts/errors to stderr. | PASS |
| III. Test-first delivery | Write failing generation, CLI, Compose-template, and real Compose integration tests before implementation; proceed Red-Green-Refactor. | PASS |
| IV. Contract and integration verification | Add contract coverage for the generator and Compose input contract; add a Docker-backed integration test for start, authentication, and persistence. | PASS |
| V. Observable, simple, compatible systems | Preserve structured Bookmarker logs, add non-secret startup diagnostics only, avoid new repository/UoW patterns, and record a backward-compatible minor release impact for the 0.x library. | PASS |
| Architecture and secret constraints | Database password remains environment-only; only a salted Argon2id verifier enters generated `0600` YAML; no secrets are committed or logged. | PASS |

## Release and Validation Strategy

- **Versioning impact**: Backward-compatible feature on Bookmarker `0.1.0`; increment MINOR before release and add a changelog entry. No migration guide is required because no public contract is broken.
- **Library boundary**: `bookmarker/usecase` owns Argon2id verifier generation and verification compatibility. `cmd/bookmarker-hash` is the delivery adapter. Dockerfile, entrypoint, Compose YAML, and README are deployment adapters.
- **Ports and adapters**: No new domain port is needed. Existing configuration, PostgreSQL, filesystem, and browser-capture adapters are composed by the runtime image.
- **CLI protocol**: The bootstrap-only `bookmarker-hash` service is line-oriented: verifier on stdout; prompt/diagnostics on stderr; non-zero exit on invalid/empty input. Existing `bookmarker-cli` JSON Lines protocol remains unchanged.
- **Logging**: Preserve structured application logs. The entrypoint reports missing setting names, rejected unsafe input names, and non-secret lifecycle errors only; hash and password values never appear in output.
- **Test-first sequence**: Add failing password-generation unit tests; then bootstrap-service and safe-rendering contract tests; then Docker-backed dependency-failure, startup, copy-bundle, and persistence integration tests; implement the smallest change for each stage before refactoring.

## Project Structure

### Documentation (this feature)

```text
specs/002-local-compose-setup/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
bookmarker/
└── usecase/
  ├── password.go                 # Extend with Argon2id PHC generation
  └── password_test.go             # Generation/verification unit coverage

cmd/
└── bookmarker-hash/
  └── main.go                      # Interactive password-to-verifier CLI adapter

deploy/
└── compose/
  ├── Dockerfile                    # Multi-stage runtime image built from checkout root
  ├── docker-compose.yml            # Copyable web, PostgreSQL, and hash bootstrap setup
  ├── .env.example                  # Committed names/placeholders only
  ├── config.yaml.tmpl              # Non-secret runtime YAML template
  └── entrypoint.sh                 # Validate safe env, render 0600 config, exec app
README.md                            # Local Compose quick-start and adaptation guide
CHANGELOG.md                         # Backward-compatible 0.x feature release note

tests/
├── contract/
│   ├── password_hash_command_test.go
│   └── compose_template_test.go
└── integration/
  └── compose_setup_test.go
```

**Structure Decision**: Extend the existing Go use-case and command layout for the reusable Argon2id generator. Package every deployment artifact in `deploy/compose/`, which is the portable unit copied within a Bookmarker source checkout; tests follow the existing contract/integration directories.

## Complexity Tracking

No constitution violations or additional architectural patterns require justification.
