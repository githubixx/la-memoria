---
title: Screenshot maintenance
type: component
sources: [S002, S003, S004, S005]
updated: 2026-09-02
---

# Screenshot maintenance

## Replacement

Updating a bookmark preserves its original `CreatedAt`; the use case validates
the mutable fields and constructs a separate update value with a new
`UpdatedAt`. (S002)

A submitted capture must be usable by the requesting session for the edited
URL, belong to the bookmark, and be consumable before its staged screenshot is
promoted. (S002)

The replacement file is promoted before the transactional database update. If
that update fails, the newly promoted file is removed as compensation. (S002)

The PostgreSQL update replaces tags by clearing bookmark-tag links and
inserting the normalized requested tags in the same transaction as bookmark
and screenshot metadata changes. (S005)

After a successful screenshot replacement, failure to remove the old file
creates a durable `remove_replaced` cleanup record instead of undoing the
committed bookmark update. (S002)

## Deletion and cleanup

Deletion requires explicit confirmation. It quarantines an existing screenshot
before deleting the bookmark and restores that screenshot if the database
deletion fails. (S003)

After a successful delete, a failed quarantine-file removal creates a durable
`remove_deleted` cleanup record and reports cleanup as pending. (S003)

Cleanup workers atomically claim due `pending` records with `FOR UPDATE SKIP
LOCKED`, marking them `running` before attempting file removal. (S005)

Successful cleanup records become `succeeded`. A failed attempt records
`cleanup_failed`, waits one minute per attempt count, and becomes the
operator-visible terminal state `abandoned` at the configured maximum attempt
count (default three). (S004)

The authenticated delivery behavior is described in [HTTP bookmark
maintenance](./http-bookmark-maintenance.md); the exercised browser workflow
is recorded in [US3 live validation](./us3-live-validation.md). (S001)
