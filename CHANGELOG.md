# Changelog

## 0.3.0

- Adopt embedded Chroma UI themes with a deployment-wide System default and
  administrator theme selection. System follows the device color preference.
- Upgrade Chroma UI to v0.3.0, adding the Leather Style Black and Red themes
  and updating Aurora Glass styling.
- Show available bookmark screenshots in list and search results, and
  improve their layout alongside bookmark details.
- Preserve existing tags when editing bookmarks, including when adding or
  removing tags.
- Use an accessible upper-right navigation menu across viewport sizes.
- Preserve the full Compose configuration in an owner-only named volume.
  Existing Compose installations must export the old container configuration
  **before** restarting or recreating it and perform the one-time import
  described below. Already-lost ephemeral settings are not recoverable.
- Configuration updates now validate the complete candidate and use a
  cross-process lock and atomic commit. Unavailable saved themes fall back
  visually to System and require an explicit replacement before saving.

### Migration notes

- Exported Go configuration/dependency structs gained fields. Replace
  unkeyed struct literals with keyed literals and update any integrations
  implementing configuration/catalog boundaries.
- Integrations using the PostgreSQL bookmark query store can use
  `NewBookmarkQueryStoreWithScreenshots` with a screenshot availability
  checker to expose available screenshots in browse and search results.
- JSONL `v1` configuration responses now include `theme`; strict response
  consumers must accept the new field. Existing clients that omit `theme`
  from validate/update retain the current valid selection (or System when
  absent). Explicit null, empty, wrong-type or unknown IDs are rejected;
  an obsolete retained ID must be replaced explicitly.
- Compose credentials now belong in the owner-only `.env.credentials` file,
  separate from non-secret `.env` interpolation settings. Move existing
  credential entries before using the updated Compose file.

#### Upgrading an existing Compose deployment

Before using the updated Compose file, move `BOOKMARKER_ADMIN_USERNAME`,
`BOOKMARKER_ADMIN_PASSWORD_HASH`, and `BOOKMARKER_DB_PASSWORD` from
`deploy/compose/.env` to an owner-only `deploy/compose/.env.credentials`,
retaining their literal values. Remove them from `.env` but keep any port or
volume overrides there. The credentials file is not migrated automatically.
Leaving the verifier in `.env` causes Compose to interpolate its dollar signs.

Previous Compose versions generated `/run/bookmarker/config.yaml` inside the
container on every start. **Before restarting, recreating, or upgrading the
old container**, stop all configuration changes and securely export its live
file. Settings already lost in an earlier recreation cannot be recovered.
For example, from this checkout:

```sh
umask 077
mkdir -p "$HOME/bookmarker-upgrade"
chmod 700 "$HOME/bookmarker-upgrade"
docker compose -f deploy/compose/docker-compose.yml cp \
  bookmarker:/run/bookmarker/config.yaml "$HOME/bookmarker-upgrade/config.yaml"
chmod 600 "$HOME/bookmarker-upgrade/config.yaml"
```

Keep a separate owner-only backup for rollback, together with the matching
database and screenshot backups. The exported file contains a password
verifier; do not commit, publish, or include it in a Docker build context.
To import once into a **new, empty** configuration volume, create a temporary
Compose override outside the checkout with an absolute export path:

```yaml
services:
  bookmarker:
    environment:
      BOOKMARKER_CONFIG_IMPORT_PATH: /import/config.yaml
    volumes:
      - type: bind
        source: /absolute/path/to/owner-only/config.yaml
        target: /import/config.yaml
        read_only: true
```

Start the upgrade using both files (substitute the actual override path):

```sh
docker compose -f deploy/compose/docker-compose.yml \
  -f /absolute/path/to/import.override.yaml up --build --detach
```

The initializer validates the import, preserves editable settings, and
refreshes only the administrator username and verifier from the deployment
environment. An explicitly requested missing, unreadable, invalid, or
non-owner-only import stops startup without creating a default file. Correct
the source and retry. An import into an already initialized volume is refused
without modifying it. Setting the variable in `.env` alone does not pass it
to the container; use the temporary override above.

After a successful import, remove **both** the import environment setting and
read-only mount by no longer using the temporary override; recreate the
service with the base Compose file before the next restart:

```sh
docker compose -f deploy/compose/docker-compose.yml up --detach --force-recreate bookmarker
```

Verify the saved configuration and keep the backup until the upgrade is
accepted. For rollback, stop configuration changes, restore the matching
database/screenshots and securely supply the saved configuration to the
previous release; do not blindly overwrite a live volume. `down --volumes`
destroys all named volumes and must not be used as an upgrade step.

## 0.0.1

Initial Bookmarker library, HTTP contract `v1`, CLI JSON Lines protocol `v1`,
and PostgreSQL schema migration `0001`. The initial Git tag was `0.0.1`.
The tagged files incorrectly identified the library as `0.2.0` and the
changelog as `0.1.0`; those labels were not separate releases.

While the library is below `1.0.0`, breaking public API changes require a
MINOR version increment and migration notes. Stable releases require a MAJOR
increment and a migration guide for breaking changes.
