# Retention Red-Green Evidence

## Reviewed Failing Checks

- `go test ./bookmarker/usecase -run TestRetentionProcessorHonorsCanceledContextBeforeStorage` initially failed with `process error = <nil>, want context canceled`, proving the processor did not honor cancellation before storage work.
- `go test ./bookmarker/usecase -run TestRetentionProcessorProcessesDueScreenshotCleanupRecords` initially failed to compile because `RetentionDependencies` did not expose a cleanup store or file store.
- `go test -tags=integration ./tests/integration -run TestPostgreSQLRetentionProcessesScreenshotCleanupRetriesWithoutDuplicateClaims` initially failed because `RetentionStore` did not implement `ports.ScreenshotCleanupStore`.

## Green Checks

- `go test ./bookmarker/usecase -run Retention` passed after the processor added cancellation checks and bounded screenshot-cleanup processing.
- `go test -tags=integration ./tests/integration -run 'TestPostgreSQLRetention(ExpiresOnlyDueRecordsInBoundedConcurrentBatches|ProcessesScreenshotCleanupRetriesWithoutDuplicateClaims)'` passed, exercising PostgreSQL `FOR UPDATE SKIP LOCKED`, durable retry state, bounded batches, and future-record preservation.
