# noinspection SqlResolveForFile
# noinspection SqlWithoutWhereForFile

ALTER TABLE tracks
    ADD COLUMN title TEXT NOT NULL,
    ADD COLUMN original_url TEXT NOT NULL,
    ADD COLUMN url TEXT NOT NULL,
    ADD COLUMN http_headers JSON NULL,
    ADD COLUMN live BOOL NOT NULL;

--bun:split

UPDATE tracks SET
    title=track_data->>'$.title',
    original_url=track_data->>'$.originalURL',
    url=track_data->>'$.url',
    http_headers=JSON_VALUE(track_data, '$.httpHeaders' RETURNING JSON),
    live=JSON_VALUE(track_data, '$.live' RETURNING SIGNED DEFAULT false ON EMPTY);

--bun:split

ALTER TABLE tracks DROP COLUMN track_data;
