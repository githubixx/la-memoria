# Tasks: Local Docker Compose Setup

**Input**: Design documents from `/specs/002-local-compose-setup/`

**Prerequisites**: [plan.md](plan.md), [spec.md](spec.md), [research.md](research.md), [data-model.md](data-model.md), [contracts/](contracts/), [quickstart.md](quickstart.md)

**Tests**: Tests are required by the project constitution. Write each task's failing test before its implementation task, confirm it fails for the intended reason, then use the smallest implementation needed to make it pass.

**Organization**: Tasks are grouped by user story to enable independent implementation, test, and demonstration.

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Establish source-checkout deployment-bundle locations and commands without introducing secrets or runtime behavior.

- [X] T001 Create the `deploy/compose/` and `cmd/bookmarker-hash/` directories according to `specs/002-local-compose-setup/plan.md`
- [X] T002 [P] Add ignored `deploy/compose/.env` handling and a committed placeholder-only `deploy/compose/.env.example` in `.gitignore` and `deploy/compose/.env.example`
- [X] T003 [P] Add a `compose-integration` test command that runs `tests/integration/compose_setup_test.go` with Docker prerequisites in `Makefile` and `package.json`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Define the shared test harness, deployment-bundle contract, and safe runtime configuration boundary required by all local Compose workflows.

**⚠️ CRITICAL**: Complete this phase before starting user-story implementation.

- [X] T004 Add Docker-backed Compose test helpers for isolated project names, temporary bundle copies, environment files, log capture, readiness polling, and guaranteed teardown in `tests/testkit/compose.go`
- [X] T005 Add failing contract coverage for complete `deploy/compose/` bundle contents, private PostgreSQL exposure, default named volumes, required inputs, rejection of malformed but YAML-safe password verifiers and unsafe YAML values, and absence of committed plaintext secrets in `tests/contract/compose_template_test.go`
- [X] T006 Add the non-secret Bookmarker runtime configuration template with internal PostgreSQL, screenshot root, and container listen defaults in `deploy/compose/config.yaml.tmpl`
- [X] T007 Add the normal-service entrypoint in `deploy/compose/entrypoint.sh` to validate required environment names and safe single-line administrator input, render mode-`0600` `config.yaml`, and execute Bookmarker without logging secrets

**Checkpoint**: The source-checkout bundle contract and secure runtime configuration boundary are ready. User-story work can begin.

---

## Phase 3: User Story 1 - Start a Local Bookmarker Instance (Priority: P1) 🎯 MVP

**Goal**: Start Bookmarker, PostgreSQL, browser capture, migrations, and persistent storage through the `deploy/compose/` bundle so an evaluator can browse, authenticate, create a bookmark, and retain it after a restart.

**Independent Test**: Copy the bundle into an isolated Bookmarker source checkout, supply a known valid verifier and database password, build and start the stack, wait for health, sign in, create a bookmark and screenshot, stop/start it, and verify both persist.

### Tests for User Story 1

- [X] T008 [US1] Add failing Docker integration coverage for bundle-based `docker compose up`, PostgreSQL health ordering, Bookmarker HTTP readiness, automatic migrations, and default host reachability in `tests/integration/compose_setup_test.go`
- [ ] T009 [US1] Add failing Docker integration coverage that creates a bookmark and screenshot, restarts the bundle stack, and verifies both persisted stores in `tests/integration/compose_setup_test.go`
- [ ] T010 [US1] Add failing Docker integration cases that assert an unavailable PostgreSQL dependency and a host-port conflict keep Bookmarker unavailable with non-secret actionable diagnostics in `tests/integration/compose_setup_test.go`

### Implementation for User Story 1

- [X] T011 [US1] Add `deploy/compose/Dockerfile` to build `bookmarker-web`, copy migrations and `web/` assets from the source-checkout context, and install pinned `agent-browser` 0.35.2 with its browser runtime
- [X] T012 [US1] Add `deploy/compose/docker-compose.yml` with normal Bookmarker and private PostgreSQL services, source-checkout-relative build context, readiness dependencies, HTTP health check, configurable host port, and default named data/screenshot volumes
- [ ] T013 [US1] Run startup, persistence, dependency-failure, and host-port-conflict integration coverage in `tests/integration/compose_setup_test.go`, correct affected bundle artifacts in `deploy/compose/`, and rerun until green

**Checkpoint**: The copied source-checkout bundle starts a healthy local stack, diagnoses an unavailable dependency safely, and retains data through restart.

---

## Phase 4: User Story 2 - Configure a Safe Local Administrator (Priority: P2)

**Goal**: Let local users generate a compatible administrator verifier without writing a plaintext password to configuration, output, or logs.

**Independent Test**: With no `.env` file present, run the bootstrap hash service with a chosen password, place its output in the environment file, start the P1 stack, and authenticate with the original plaintext password.

### Tests for User Story 2

- [X] T014 [P] [US2] Add failing unit tests for fresh-salt Argon2id PHC generation, successful verification, and rejection of empty passwords in `bookmarker/usecase/password_test.go`
- [X] T015 [P] [US2] Add a failing CLI contract test for interactive input, verifier-only stdout, non-secret stderr, and non-zero empty-password failure in `tests/contract/password_hash_command_test.go`
- [X] T016 [US2] Add a failing Docker integration test that runs `bookmarker-hash` with no `.env` file and verifies its generated verifier authenticates the normal stack in `tests/integration/compose_setup_test.go`

### Implementation for User Story 2

- [X] T017 [US2] Implement cryptographically random-salt Argon2id PHC generation with `v=19,m=65536,t=3,p=1` and the existing verifier-compatible base64 format in `bookmarker/usecase/password.go`
- [X] T018 [US2] Implement the interactive line-oriented `bookmarker-hash` command, with prompts and diagnostics on stderr and exactly one verifier line on stdout, in `cmd/bookmarker-hash/main.go`
- [X] T019 [US2] Add the bootstrap-only `bookmarker-hash` service with no PostgreSQL dependency or required normal-service environment inputs in `deploy/compose/docker-compose.yml`
- [X] T020 [US2] Include the `bookmarker-hash` binary in the runtime image and keep it directly executable by the bootstrap service in `deploy/compose/Dockerfile`
- [X] T021 [US2] Add README instructions to run the bootstrap hash service, create the local environment file, avoid plaintext password storage, and verify administrator sign-in in `README.md`

**Checkpoint**: The dedicated hash service works before normal configuration and the verifier safely authenticates a local Bookmarker instance.

---

## Phase 5: User Story 3 - Adapt the Compose Template (Priority: P3)

**Goal**: Make `deploy/compose/` a clear, copyable source-checkout deployment bundle whose host port, identity, secrets, and named persistent volumes can be adjusted without undocumented internal changes.

**Independent Test**: Copy the complete `deploy/compose/` directory into a separate Bookmarker source checkout, set a non-default host port and named-volume values in `.env`, start it, and verify reachability, internal database connectivity, and restart persistence.

### Tests for User Story 3

- [X] T022 [P] [US3] Add failing contract coverage that every Compose environment input, volume, and host-port override is documented and that no PostgreSQL host port is published in `tests/contract/compose_template_test.go`
- [X] T023 [US3] Add a failing Docker integration test that copies the complete bundle into an isolated source checkout, changes host-port and named-volume values, and verifies reachability and persistence in `tests/integration/compose_setup_test.go`

### Implementation for User Story 3

- [X] T024 [US3] Add template comments and variable expansion for host-port, PostgreSQL-volume, and screenshot-volume names while preserving internal defaults in `deploy/compose/docker-compose.yml`
- [X] T025 [US3] Complete placeholder-only required-input, named-volume override, and safe-copy guidance in `deploy/compose/.env.example`
- [X] T026 [US3] Expand the README with source-checkout bundle copy, bundle-scoped lifecycle commands, readiness checks, host-port conflict resolution, named-volume overrides, reset behavior, and non-local cautions in `README.md`
- [X] T027 [US3] Run bundle contract and copied-bundle integration tests in `tests/contract/compose_template_test.go` and `tests/integration/compose_setup_test.go`; correct only affected bundle or README files until green

**Checkpoint**: The checked-in files form a safe, understandable copyable template with defaults for evaluation and documented adjustments for local variation.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Complete release, documentation, regression, and manual acceptance evidence without expanding the feature scope.

- [X] T028 [P] Bump Bookmarker from `0.1.0` to `0.2.0` in `go.mod` and `package.json`, and add the matching backward-compatible Docker Compose onboarding and `bookmarker-hash` release note in `CHANGELOG.md`
- [X] T029 [P] Document the local Compose configuration boundary and cross-reference the README setup in `docs/configuration.md`
- [ ] T030 Add executed quickstart evidence for default startup, generated-verifier sign-in, restart persistence, host-port override, and expected failures in `specs/002-local-compose-setup/validation.md`
- [X] T031 Run Go formatting and all feature-scoped test commands from `Makefile` against `bookmarker/usecase/password_test.go`, `tests/contract/password_hash_command_test.go`, `tests/contract/compose_template_test.go`, and `tests/integration/compose_setup_test.go`
- [ ] T032 Run the full repository validation suite and record any unrelated pre-existing failures separately in `specs/002-local-compose-setup/validation.md` using `make validate`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies; T001 precedes artifacts that need its directories. T002 and T003 may proceed in parallel afterward.
- **Foundational (Phase 2)**: Depends on T001-T003. T004-T006 can proceed in parallel; T007 depends on T006.
- **User Story 1 (Phase 3)**: Depends on T004-T007. Write and confirm the failing T008-T010 integration cases before implementing T011-T012; T013 validates the completed slice.
- **User Story 2 (Phase 4)**: Depends on the completed P1 runtime image and Compose stack from T011-T013 plus foundational configuration from T006-T007. T014-T016 can proceed in parallel; T017 precedes T018; T019-T020 follow the failing bootstrap integration test in T016 and the corresponding CLI implementation; T021 completes the documented workflow.
- **User Story 3 (Phase 5)**: Depends on the P1 Compose template. T022-T023 can proceed in parallel; T024-T026 follow; T027 validates the completed slice.
- **Polish (Phase 6)**: Depends on the desired user stories. T028 and T029 may run in parallel; T030-T032 follow the completed feature validation.

### User Story Dependencies

- **User Story 1 (P1)**: The MVP; has no dependency on other user stories once the foundation is complete.
- **User Story 2 (P2)**: Requires the completed P1 runtime image and Compose stack (T011-T013) to provide the user-facing `docker compose run` hash command, but its library and CLI tests can be written independently.
- **User Story 3 (P3)**: Requires the P1 Compose template and validates its copy-and-adjust contract; it does not depend on the password generator's implementation.

### Dependency Graph

```text
Setup -> Foundational -> US1 (MVP) -> US2 -> US3 -> Polish
                         |             |
                         |             +-> secure verifier onboarding
                         +-> runnable persistent local stack
```

## Parallel Opportunities

### User Story 1

```text
T008: startup and health integration test in tests/integration/compose_setup_test.go
T009: persistence integration test in tests/integration/compose_setup_test.go
T010: dependency-failure and host-port-conflict integration tests in tests/integration/compose_setup_test.go
```

T008-T010 share one test file and must be coordinated as a single test-first edit; after their cases are established, Dockerfile and Compose implementation work can proceed alongside README work planned for US2.

### User Story 2

```text
T014: generator unit tests in bookmarker/usecase/password_test.go
T015: command contract test in tests/contract/password_hash_command_test.go
T016: generated-verifier integration test in tests/integration/compose_setup_test.go
```

### User Story 3

```text
T022: template documentation contract test in tests/contract/compose_template_test.go
T023: copied-template integration test in tests/integration/compose_setup_test.go
```

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Setup and Foundational tasks through T007.
2. Write and confirm the P1 integration tests fail (T008-T010).
3. Implement the image, Compose services, health checks, volumes, and runtime paths (T011-T012).
4. Run T013 and manually complete the P1 quickstart readiness and persistence checks.
5. Stop here for a deployable local-evaluation MVP.

### Incremental Delivery

1. Deliver US1 as a fully persistent local stack using an already known valid test verifier.
2. Add US2 to replace external/manual hash creation with the safe built-in `bookmarker-hash` workflow.
3. Add US3 to verify the template remains easy to copy and tune, including host-port and storage guidance.
4. Finish release notes, documentation cross-references, quickstart evidence, and the full quality gate.

### Format Validation

All 32 tasks use the required `- [ ] T### [P?] [US?] Description with file path` checklist format. Story labels are present on every user-story task and absent from setup, foundational, and polish tasks.
