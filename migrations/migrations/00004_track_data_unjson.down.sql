# noinspection SqlResolveForFile
# noinspection SqlWithoutWhereForFile

ALTER TABLE tracks ADD COLUMN track_data JSON NOT NULL;

--bun:split

UPDATE tracks SET
    track_data=JSON_OBJECT(
        'title', title,
        'originalURL', original_url,
        'url', url,
        'httpHeaders', http_headers,
        'live', IF(live, CAST(TRUE AS JSON), CAST(FALSE AS JSON))
    );

--bun:split

ALTER TABLE tracks
    DROP COLUMN title,
    DROP COLUMN original_url,
    DROP COLUMN url,
    DROP COLUMN http_headers,
    DROP COLUMN live;
