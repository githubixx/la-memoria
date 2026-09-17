CREATE TABLE schema_migrations (
    version TEXT NOT NULL
);
INSERT INTO schema_migrations (version) VALUES ('0001');

CREATE TABLE bookmarks (
    id TEXT PRIMARY KEY,
    url TEXT NOT NULL,
    description TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX bookmarks_created_at_id_desc_idx ON bookmarks (created_at DESC, id DESC);
CREATE INDEX bookmarks_description_search_idx ON bookmarks USING GIN (to_tsvector('simple', description));

CREATE TABLE tags (
    id TEXT PRIMARY KEY,
    display_name TEXT NOT NULL,
    normalized_name TEXT NOT NULL UNIQUE
);

CREATE TABLE bookmark_tags (
    bookmark_id TEXT NOT NULL REFERENCES bookmarks (id) ON DELETE CASCADE,
    tag_id TEXT NOT NULL REFERENCES tags (id) ON DELETE CASCADE,
    PRIMARY KEY (bookmark_id, tag_id)
);

CREATE TABLE screenshots (
    id TEXT PRIMARY KEY,
    bookmark_id TEXT NOT NULL UNIQUE REFERENCES bookmarks (id) ON DELETE CASCADE,
    storage_key TEXT NOT NULL UNIQUE,
    captured_url TEXT NOT NULL,
    captured_at TIMESTAMPTZ NOT NULL,
    media_type TEXT NOT NULL DEFAULT 'image/png',
    byte_size INTEGER NOT NULL
);

CREATE TABLE web_sessions (
    id TEXT PRIMARY KEY,
    token_digest BYTEA NOT NULL UNIQUE,
    csrf_token_digest BYTEA NOT NULL,
    state TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    authenticated_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ
);

CREATE TABLE capture_drafts (
    id TEXT PRIMARY KEY,
    owner_session_id TEXT NOT NULL REFERENCES web_sessions (id) ON DELETE CASCADE,
    bookmark_id TEXT REFERENCES bookmarks (id) ON DELETE SET NULL,
    target_url TEXT NOT NULL,
    state TEXT NOT NULL,
    staged_key TEXT,
    final_url TEXT,
    failure_code TEXT,
    failure_message TEXT,
    created_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE screenshot_cleanup (
    id TEXT PRIMARY KEY,
    screenshot_id TEXT,
    quarantined_key TEXT NOT NULL,
    operation TEXT NOT NULL,
    state TEXT NOT NULL,
    attempt_count INTEGER NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL,
    last_error_code TEXT
);

CREATE TABLE login_throttle_pairs (
    username_key BYTEA NOT NULL,
    source_address TEXT NOT NULL,
    window_started_at TIMESTAMPTZ NOT NULL,
    failure_count INTEGER NOT NULL DEFAULT 0,
    blocked_until TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (username_key, source_address)
);
