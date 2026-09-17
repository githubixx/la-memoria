# Research: Local Docker Compose Setup

## Decision: Build and run Bookmarker from a source-checkout deployment bundle

- **Decision**: Add a complete deployment bundle under `deploy/compose/`: Dockerfile, Compose file, runtime configuration template, entrypoint, and placeholder-only environment example. Its Compose build context is the containing Bookmarker source checkout, so the bundle can be copied into another checkout without undeclared files.
- **Rationale**: `bookmarker-web` requires the web templates, static assets, migrations, a writable screenshot directory, PostgreSQL, and `agent-browser` on `PATH`. Keeping all deployment inputs in one documented directory makes the supported copy unit unambiguous while allowing the image to package application source from the checkout.
- **Alternatives considered**:
  - Copy only a root Compose file and environment example: rejected because the Docker build also requires image and runtime-template artifacts.
  - Require a prebuilt Bookmarker image: rejected because no image publication workflow currently exists and it would make a fresh checkout dependent on external release delivery.
  - Mount source code and run `go run`: rejected because it requires Go, Node.js, and a host-installed `agent-browser`, which conflicts with the Docker-only onboarding goal.
  - Embed assets with Go: rejected because it changes existing runtime asset and configuration conventions without being needed for the Compose feature.

## Decision: Render safe runtime YAML configuration at container startup

- **Decision**: Commit a non-secret configuration template to the image and use a small entrypoint to validate constrained, YAML-safe administrator input values before substitution, render a runtime `config.yaml` with mode `0600`, and start the requested Bookmarker binary. The generated file is ephemeral and is not mounted from the user checkout.
- **Rationale**: Bookmarker validates YAML directly and does not expand environment placeholders in configuration values. Its configuration loader also rejects files with group or other access. Restricting administrator input to a safe single-line grammar avoids unsafe shell/YAML interpolation without adding a second renderer binary.
- **Alternatives considered**:
  - Commit a ready-to-run `config.yaml`: rejected because it would require a committed administrator verifier and conflicts with the selected no-default-secrets policy.
  - Pass all application settings as environment variables: rejected because Bookmarker is intentionally configured through one strict YAML file.
  - Ask the user to manually create a `0600` config before Compose startup: rejected because it adds avoidable onboarding work and makes the copied template less self-contained.
  - Render YAML through an additional helper program: rejected because constrained username and PHC verifier values can be safely validated by the existing entrypoint with less deployment complexity.

## Decision: Use one private PostgreSQL service and named persistent volumes

- **Decision**: The Compose template defines private PostgreSQL, Bookmarker web, and bootstrap-only password-hash services. PostgreSQL data and screenshots use separately configurable named-volume names by default; Bookmarker alone maps a configurable host port. The database service is not published to the host.
- **Rationale**: This meets the local evaluation workflow while retaining records and screenshots over `docker compose down`/`up` cycles. Named volumes provide persistence without host-directory permissions or repository-relative paths, and configurable names give copied deployments a supported storage-isolation adjustment. Internal Docker DNS supplies the database host.
- **Alternatives considered**:
  - Publish PostgreSQL on a host port: rejected because it is not needed for normal local use and broadens the host exposure surface.
  - Use bind mounts by default: rejected because their path and ownership behavior varies across host operating systems.
  - Store screenshots only in the application container layer: rejected because captured files would be lost when the container is recreated.

## Decision: Rely on PostgreSQL health and Bookmarker HTTP readiness

- **Decision**: Configure the database health check with the image-provided readiness probe and delay Bookmarker startup until it is healthy. Add a Bookmarker HTTP health check that succeeds only after its configuration, PostgreSQL connection, and automatic migrations have completed and the server is listening.
- **Rationale**: The web executable connects to PostgreSQL, runs migrations, initializes filesystem storage, and only then begins serving HTTP. An HTTP response from the application therefore represents usable readiness better than a mere open port.
- **Alternatives considered**:
  - Start both services without dependency readiness: rejected because Bookmarker currently exits when the database is temporarily unavailable during startup.
  - Add a new application health endpoint: rejected because an existing public page can provide the required local readiness signal without widening the application API.
  - Use a database port check only: rejected because a listening port is insufficient during PostgreSQL initialization.

## Decision: Add a focused password-hash CLI and bootstrap Compose service

- **Decision**: Add a small `bookmarker-hash` command backed by a use-case password-hash generator and expose it through a dedicated `bookmarker-hash` Compose service. The bootstrap service shares the built image but has no database, administrator, or normal-service environment requirements.
- **Rationale**: Existing code verifies Argon2id PHC values but cannot generate them. A dedicated service lets the documented command run before `.env` exists, while a focused command honors the constitution's composable CLI requirement, keeps plaintext out of YAML and output, and produces values compatible with Bookmarker validation.
- **Alternatives considered**:
  - Document a third-party language package or online generator: rejected because it adds an undeclared host dependency or risks disclosing a password.
  - Add a JSON Lines operation to `bookmarker-cli`: rejected because its startup requires a valid configuration and database, which users do not yet have when creating the verifier.
  - Accept the password as a command-line argument: rejected because process listings and shell history can expose it.

## Decision: Generate compatible Argon2id PHC values

- **Decision**: Use `crypto/rand` for a fresh salt and `argon2.IDKey` with the existing accepted format: version 19, memory 65536, time cost 3, parallelism 1, and a 32-byte derived key; encode salt and key with base64 RawStdEncoding.
- **Rationale**: `VerifyPassword` already parses this format and parameter set. Pairing generation and verification in the use-case package provides one tested source of truth.
- **Alternatives considered**:
  - Loosen the verifier to accept arbitrary Argon2id parameters: rejected because it changes the existing configuration contract without an onboarding need.
  - Hard-code salt or key material for reproducible examples: rejected because each password verifier requires a fresh cryptographic salt.

## Decision: Test the public Compose and hash contracts at their boundaries

- **Decision**: Add unit tests for password generation, contract tests that exercise the hash binary and inspect the bundle/Compose/documentation contract, and opt-in Compose integration tests for bootstrap generation, unavailable PostgreSQL diagnostics, normal startup, authentication, and persistence.
- **Rationale**: The constitution requires test-first delivery and real-boundary integration verification for service contracts. Parsing alone cannot prove that a no-secret bootstrap command works, that dependency failure is actionable, or that the built image, migrations, database, authentication, and persistence operate together.
- **Alternatives considered**:
  - Unit-test only the template rendering: rejected because it would not validate the actual Compose environment.
  - Run the full Compose test in every unit test command: rejected because image build and browser runtime make it materially slower and it needs Docker; an explicit integration target keeps the normal suite focused.
