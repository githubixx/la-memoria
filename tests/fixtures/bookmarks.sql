-- Deterministic E2E fixture data for User Story 1 (Browse and Find Bookmarks).
-- Seeds duplicate URLs, mixed-case tag spellings, a bookmark with no
-- screenshot, a capped-search target phrase, and enough bookmarks (500) for
-- 50 list pages at the fixed 10-item page size.

INSERT INTO tags (id, display_name, normalized_name) VALUES
    ('11111111-1111-1111-1111-111111111111', 'Go', 'go'),
    ('22222222-2222-2222-2222-222222222222', 'Rust', 'rust');

-- Two intentionally duplicate URLs with different descriptions and tags;
-- neither has a screenshot, exercising the missing-screenshot placeholder.
INSERT INTO bookmarks (id, url, description, created_at, updated_at) VALUES
    ('a0000000-0000-0000-0000-000000000001', 'https://example.test/duplicate', 'First entry for a duplicated URL', now() - interval '600 hours', now() - interval '600 hours'),
    ('a0000000-0000-0000-0000-000000000002', 'https://example.test/duplicate', 'Second entry for the same duplicated URL', now() - interval '599 hours', now() - interval '599 hours');
INSERT INTO bookmark_tags (bookmark_id, tag_id) VALUES
    ('a0000000-0000-0000-0000-000000000001', '11111111-1111-1111-1111-111111111111'),
    ('a0000000-0000-0000-0000-000000000002', '22222222-2222-2222-2222-222222222222');

-- 498 additional filler bookmarks (500 total) so browse/search exercise a
-- full 50-page pagination control; every fourth is tagged "go" and mentions
-- "database transactions" so tag filtering, all-word search, and the
-- configured result cap all have deterministic matches.
INSERT INTO bookmarks (id, url, description, created_at, updated_at)
SELECT
    md5('filler-' || series::text),
    'https://example.test/filler/' || series,
    CASE WHEN series % 4 = 0
        THEN 'Filler bookmark about database transactions number ' || series
        ELSE 'Filler bookmark number ' || series
    END,
    now() - make_interval(hours => series),
    now() - make_interval(hours => series)
FROM generate_series(1, 498) AS series;

INSERT INTO bookmark_tags (bookmark_id, tag_id)
SELECT md5('filler-' || series::text), '11111111-1111-1111-1111-111111111111'
FROM generate_series(1, 498) AS series
WHERE series % 4 = 0;
