# Feature Specification: Local Docker Compose Setup

**Feature Branch**: `initial`

**Created**: 2026-09-02

**Status**: Draft

**Input**: User description: "We want to make it easy to get started with Bookmarker. We should therefore have a Docker Compose setup that includes everything needed to be able to test the application locally. The docker-compose file should also be usable as a template the user can copy and paste and adjust to his need. We should have sensible defaults that makes it easy to get started and doesn't require much changes if the user only wants to test the application. We also need this document in README and how to generate a password hash easily."

## Clarifications

### Session 2026-09-02

- Q: Should the default local quick-start require users to choose and provide their own database password and administrator password verifier before the first `docker compose up`? → A: Require a user-chosen database password and administrator password verifier before startup.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Start a Local Bookmarker Instance (Priority: P1)

As a prospective Bookmarker user, I can start a complete local instance with one documented Docker Compose command so that I can evaluate the application without separately provisioning its required services.

**Why this priority**: A working, low-friction local evaluation path is the feature's primary value.

**Independent Test**: On a machine with Docker Compose and no pre-existing Bookmarker services, follow the README quick-start instructions and confirm that the application opens in a browser, can authenticate the configured administrator, and persists a newly created bookmark after a restart.

**Acceptance Scenarios**:

1. **Given** Docker Compose is available and the repository is newly cloned, **When** a user follows the documented local quick-start steps, **Then** all services required to run Bookmarker locally start without the user first installing or configuring a separate database.
2. **Given** the default local setup is running, **When** the user opens the documented local address, **Then** the Bookmarker interface is reachable and ready for public browsing and administrator sign-in.
3. **Given** a user creates a bookmark and its screenshot through the local setup, **When** they stop and start the setup again, **Then** the bookmark and screenshot remain available.

---

### User Story 2 - Configure a Safe Local Administrator (Priority: P2)

As a user testing Bookmarker locally, I can generate an administrator password hash with a documented command and supply it through the Compose template so that I can sign in without storing a plaintext administrator password in configuration.

**Why this priority**: Authentication is necessary to test creation and maintenance workflows, and the supplied path must preserve Bookmarker's credential safeguards.

**Independent Test**: Generate a hash using the README command, provide it and a chosen database password through the documented local configuration mechanism, start the stack, and sign in successfully with the corresponding plaintext password.

**Acceptance Scenarios**:

1. **Given** a user chooses an administrator password, **When** they run the documented password-hash generation command, **Then** it produces a valid salted Argon2id verifier suitable for the local setup.
2. **Given** a user follows the documented configuration steps, **When** they review the Compose template and local configuration files, **Then** no plaintext administrator password is present and the database password is supplied as a runtime secret rather than committed configuration.
3. **Given** a user supplies a new administrator hash and matching plaintext password, **When** the local setup is started, **Then** they can authenticate with that password.

---

### User Story 3 - Adapt the Compose Template (Priority: P3)

As a deployer, I can copy the supplied Compose deployment bundle into a Bookmarker source checkout and adjust clearly identified values so that the same starting point supports my own local or small-scale environment.

**Why this priority**: The template should remain useful beyond a one-time evaluation while keeping the default path uncomplicated.

**Independent Test**: Copy the supplied Compose deployment bundle into a separate Bookmarker source checkout, change the documented application address, administrator identity, and persistent-volume names, then start it and confirm Bookmarker uses the supplied values and retains data across restarts.

**Acceptance Scenarios**:

1. **Given** a deployer copies the documented Compose deployment bundle into a Bookmarker source checkout, **When** they inspect its configurable values, **Then** each value they are expected to adjust is clearly named, has a sensible default or example, and is documented in the README.
2. **Given** a deployer only wants to evaluate Bookmarker, **When** they use the defaults and provide the minimum documented secrets, **Then** no changes to application source code or internal service connection settings are required.
3. **Given** a deployer changes documented external address, credentials, or storage settings, **When** they start their copied configuration, **Then** Bookmarker and its supporting services use those changed values without requiring undocumented changes.

### Edge Cases

- A requested host port is already in use; the documentation identifies how to choose an alternative without changing internal service connectivity.
- Required local secrets are missing, empty, or use the example values; startup fails with actionable guidance and does not start an insecure, misleadingly usable instance.
- The supplied administrator password hash is malformed; startup identifies the invalid configuration without revealing secret values.
- The database service is not ready when Bookmarker begins startup; Compose waits for PostgreSQL health before launching Bookmarker, and an unavailable database produces a non-secret actionable startup diagnostic.
- A user stops only the application service; restarting it reconnects to the persistent local data without requiring data reinitialization.
- A user runs the setup on an unsupported or resource-constrained container environment; the README states the local prerequisites and provides meaningful failure guidance.
- The Compose deployment bundle is copied to another Bookmarker source checkout; documented paths and volumes remain understandable and require no undeclared files beyond that checkout.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The project MUST provide a Docker Compose configuration that starts Bookmarker and every supporting service required for a user to browse, authenticate, create, and retain bookmarks locally.
- **FR-002**: The default Docker Compose configuration MUST use persistent storage for bookmark data and captured screenshots so that locally created content survives service restarts.
- **FR-003**: The default setup MUST expose Bookmarker at one documented local browser address and keep supporting services inaccessible from the host unless exposure is explicitly documented as configurable.
- **FR-004**: The default setup MUST make Bookmarker usable for local evaluation after a user supplies a chosen database password and a chosen administrator password verifier through the documented minimum configuration steps and starts Docker Compose.
- **FR-005**: The `deploy/compose/` deployment bundle MUST be usable as a copyable template within a Bookmarker source checkout, with user-adjustable values clearly identified and no undocumented build or configuration dependencies.
- **FR-006**: The template MUST provide sensible defaults for the application identity, database name, service connectivity, screenshot storage, and browser address that work together without manual reconciliation.
- **FR-007**: The template MUST allow a user to change the externally exposed application address or host port without requiring changes to internal service connectivity.
- **FR-008**: The template MUST allow a user to provide an administrator username, a valid salted Argon2id password verifier, and a database password through documented configuration inputs.
- **FR-008a**: The template MUST provide a dedicated password-hash service that runs without administrator or database configuration so users can generate a verifier before normal startup.
- **FR-009**: The template and its documented usage MUST NOT contain a plaintext administrator password, commit database credentials, or log supplied secret values.
- **FR-010**: The project README MUST include a self-contained local Docker Compose quick-start section that states prerequisites, the minimum configuration steps, the start command, the local browser address, and how to stop the setup.
- **FR-011**: The README MUST explain which local configuration values users may change, their purpose, and the effect of changing them.
- **FR-012**: The README MUST provide a copy-and-run command that generates a valid salted Argon2id password verifier from a user-selected plaintext password, together with instructions for placing the result in the local configuration.
- **FR-013**: The password-hash instructions MUST warn users not to store the plaintext administrator password in configuration or share generated verifiers unnecessarily.
- **FR-014**: The startup experience MUST detect or clearly surface missing required configuration, malformed password verifiers, unavailable service dependencies, and host-port conflicts with enough guidance for a user to correct the problem.
- **FR-014a**: Values rendered into runtime YAML MUST use a documented YAML-safe grammar or serialization method; malformed administrator identities or verifiers MUST be rejected without rendering malformed configuration or exposing values.
- **FR-015**: The local setup MUST initialize the database schema through the application's authorized migration path before accepting normal Bookmarker use.
- **FR-016**: The local setup and README MUST state that the provided configuration is intended for local testing and identify which defaults must be reviewed before use outside a local environment.
- **FR-016a**: The template MUST support documented overrides for the names of the persistent PostgreSQL and screenshot volumes while retaining named volumes as the default storage mechanism.

### Key Entities

- **Local Compose Deployment Bundle**: The copyable `deploy/compose/` directory in a Bookmarker source checkout, containing the service definition, image build instructions, runtime configuration template, entrypoint, and non-secret environment example.
- **Local Environment Settings**: User-supplied values that control administrator identity, database credential, browser address, and optional persistent storage locations without modifying application source.
- **Administrator Password Verifier**: A salted Argon2id representation of the administrator's chosen password that permits authentication without retaining the plaintext password.
- **Persistent Local Data**: Bookmark records and captured screenshots retained independently of the lifetime of individual application or database service instances.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: At least 90% of first-time evaluators who meet the stated prerequisites can open a working Bookmarker instance and sign in as administrator within 15 minutes by following only the README.
- **SC-002**: At least 90% of evaluators can generate a password verifier, configure the minimum required local secrets, and complete their first bookmark creation within 20 minutes without source-code changes.
- **SC-003**: In acceptance testing on supported container environments, 100% of default setup runs make Bookmarker reachable at the documented browser address after all required services become healthy.
- **SC-004**: In acceptance testing, 100% of bookmarks and screenshots created through the default setup remain accessible after the complete Compose stack is stopped and restarted.
- **SC-005**: In usability review, at least 90% of participants can identify how to change the host port, administrator verifier, database password, and persistent storage location from the README and copied template in under 5 minutes.
- **SC-006**: In a review of the committed template and documentation, zero plaintext administrator passwords or database passwords are present.

## Assumptions

- The target user has Docker-compatible container tooling with Docker Compose available and sufficient local resources to run Bookmarker and PostgreSQL.
- The supplied deployment bundle is an evaluation and adaptation starting point, not a production-hardening guide; users must review credentials, network exposure, backups, and transport security before non-local use.
- Bookmarker's existing configuration format, migration authorization mechanism, and Argon2id verifier requirements remain the source of truth for the local setup.
- Users choose their own administrator plaintext password and database password before startup; the project does not provide runnable default secret values.
- The deployment bundle builds from the containing Bookmarker source checkout. A standalone bundle that pulls a published versioned Bookmarker image is outside this feature.
- The local setup's default browser address may use an unencrypted local connection; this is acceptable only for the stated local-testing scope.
