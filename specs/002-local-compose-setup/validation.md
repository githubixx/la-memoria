# Validation Evidence: Local Compose Setup

## Executed Feature Checks

| Scenario | Command | Result |
| --- | --- | --- |
| Go formatting and unit tests | `make format unit` | Passed |
| Bootstrap verifier protocol | `go test -tags=contract ./tests/contract -run TestPasswordHashCommand -count=1` | Passed; stdout contains only an Argon2id verifier and rejects empty input. |
| Compose template contract | `go test -tags=contract ./tests/contract -run TestCompose -count=1` | Passed; validates private PostgreSQL, documented inputs, and secret-safe template behavior. |
| Default Compose start and health | `go test -tags=integration ./tests/integration -run TestComposeStartsAfterPostgreSQLHealthAndServesBookmarker -count=1` | Passed; copied source checkout starts at `http://127.0.0.1:<host-port>/bookmarks` after PostgreSQL becomes healthy. |
| Bootstrap without `.env` | `go test -tags=integration ./tests/integration -run TestComposeHashServiceWorksWithoutEnvironmentFile -count=1` | Passed; the bootstrap service emits a usable verifier without plaintext output. |
| Host-port and named-volume override | `go test -tags=integration ./tests/integration -run TestComposeCopiedBundleHonorsHostPortAndNamedVolumeOverrides -count=1` | Passed. |
| Feature regression suite | `make format unit contract compose-integration && docker compose -f deploy/compose/docker-compose.yml config --quiet && go vet ./...` | Passed before the final persistence/failure additions. |

## Final Acceptance Gate

`make validate` is the final repository-wide gate. In the current repository state it fails for two unrelated tests outside the local Compose feature scope:

- `TestCapturePreviewReadsReadyDraftScreenshot` in `bookmarker/usecase` fails with "capture draft was not found, want conflict".
- `TestStructuredLoggerRedactsCredentialsTokensAndRequestBodies` in `tests/contract` fails because a cookie field named `cookies` is logged without redaction.

The local Compose feature itself is validated independently with the following commands, which passed:

```sh
go test -tags=integration ./tests/integration -run 'TestComposeRetainsPostgreSQLDataAcrossBookmarkerRestart|TestComposeReportsUnavailablePostgreSQLWithoutExposingSecrets|TestComposeReportsHostPortConflict|TestComposeCopiedBundleHonorsHostPortAndNamedVolumeOverrides' -count=1
```

This returned:

```text
ok      github.com/githubixx/la-memoria/tests/integration       46.063s
```

The full repository gate is still red because of those unrelated failures, but the Compose acceptance checks for startup, restart persistence, PostgreSQL dependency failure, and host-port conflict all passed as part of the feature-scoped validation.

## Expected Safe Failures

- Missing required normal-service input: the container exits with a named required-setting diagnostic and does not echo a supplied database password.
- Unavailable PostgreSQL: the normal service exits instead of serving HTTP and reports a PostgreSQL connection failure without a secret value.
- Occupied host port: Compose does not publish Bookmarker and reports the address/port binding error; select another `BOOKMARKER_HOST_PORT` value.
