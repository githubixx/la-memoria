<!--
Sync Impact Report
- Version change: 2.0.0 -> 3.0.0
- Modified principles:
  - V. Observable, Simple, and Compatible Systems: initial-development
    releases may introduce breaking changes through a MINOR bump
- Rationale:
  - Libraries below 1.0.0 explicitly do not promise a stable public API
  - Breaking initial-development changes still require a visible version
    increment and migration notes
- Migration impact:
  - Breaking changes to 0.y.z libraries increment MINOR
  - Breaking changes to libraries at or above 1.0.0 increment MAJOR
  - Existing stable-release compatibility requirements remain unchanged
- Follow-up TODOs:
  - Re-run constitution checks for active feature plans
-->
# la-memoria Constitution

## Core Principles

### I. Library-First Architecture

Every feature MUST begin as a standalone library with a clear, independently testable purpose.
Application code MUST NOT implement feature behavior directly unless its plan's Complexity Tracking
section records a specific justification. Each library MUST apply Hexagonal Architecture using
ports and adapters, with a Clean Architecture use-case layer that orchestrates those adapters.
This keeps feature logic reusable, testable, and independent of delivery mechanisms.

### II. Composable Command-Line Interfaces

Every library MUST expose its functionality through a CLI that uses defined stdin/stdout protocols.
CLIs MUST preserve machine-composable output on stdout and send diagnostics and errors to stderr.
Their text I/O contracts MUST be documented and debuggable so libraries can be reliably combined
in scripts and automation.

### III. Test-First Delivery

Test-driven development is mandatory. Before implementation code is written, tests MUST be written,
reviewed, and confirmed failing. Changes MUST follow the Red-Green-Refactor cycle, with the
smallest implementation needed to make the approved failing test pass before refactoring.
This produces executable requirements and prevents unverified behavior from entering the system.

### IV. Contract and Integration Verification

Integration tests are REQUIRED for new library contracts, contract changes, inter-service
communication, and shared schema changes. Such tests MUST exercise the real boundary contract
rather than only mocks. They protect compatibility where isolated unit tests cannot establish it.

### V. Observable, Simple, and Compatible Systems

All services MUST emit structured logs. Text I/O MUST retain sufficient diagnostic context, and
frontend logs MUST be unified with backend logs when those components participate in the same
workflow. Implementations MUST begin with the simplest solution that meets current requirements;
patterns such as Repository and UnitOfWork MUST NOT be introduced without an explicit necessity
and a recorded complexity justification. Libraries MUST use semantic versioning. While a library
remains in initial development with a `0.y.z` version, a breaking change MUST increment the MINOR
version and include migration notes; the public API MUST NOT be represented as stable. After the
library reaches `1.0.0`, every breaking change MUST increment the MAJOR version and include a
migration guide. Backward-compatible features increment MINOR for stable releases, and
backward-compatible fixes increment PATCH.

## Architecture and Technology Constraints

Browser-executed code MUST prefer vanilla HTML, CSS, and JavaScript unless a documented requirement
demonstrates that another technology is necessary. Projects MUST prefer established, portable,
well-understood technologies and MUST avoid vendor lock-in where a practical portable alternative
exists. Secrets MUST be provided through environment variables and MUST NOT be committed, embedded
in source, or logged.

As a narrowly scoped exception, a deployment-owned configuration file MAY contain a salted,
one-way password verifier when an approved feature specification explicitly requires
configuration-based authentication. The configuration file MUST NOT contain the plaintext
password, MUST NOT be committed to version control, and MUST be protected with restrictive
filesystem permissions. The verifier MUST use an approved password-hashing algorithm with a
per-password salt, MUST NOT be exposed through application interfaces or operational records,
and MUST be rotated only through the deployment environment. This exception does not apply to
database credentials, session keys, tokens, encryption keys, or other runtime secrets, which
MUST remain environment-provided.

## Development Workflow and Quality Gates

Every feature plan MUST identify its library boundary, ports, adapters, use cases, CLI protocol,
test strategy, logging behavior, versioning impact, and any required integration tests. Plans MUST
include a Complexity Tracking section. Any exception to library-first implementation or any added
architectural pattern MUST state the problem, alternatives considered, and why the added complexity
is necessary. Reviewers MUST verify that failing tests preceded implementation and that required
integration tests, structured logging, semantic-versioning decisions, and migration guides are
present before approval.

## Governance

This constitution supersedes conflicting development practices. Amendments MUST document the
affected principles, rationale, migration impact, and semantic version bump. A MAJOR version
increments for incompatible governance removals or redefinitions, a MINOR version increments for
new principles or materially expanded requirements, and a PATCH version increments for clarifying
or non-semantic changes. Each plan, implementation review, and release review MUST assess
compliance; unresolved exceptions require a documented Complexity Tracking entry and explicit
approval before work proceeds.

**Version**: 3.0.0 | **Ratified**: 2026-08-06 | **Last Amended**: 2026-08-31
