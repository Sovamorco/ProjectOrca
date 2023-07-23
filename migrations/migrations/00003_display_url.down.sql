# noinspection SqlResolveForFile

ALTER TABLE tracks DROP COLUMN display_url;

--bun:split

ALTER TABLE tracks RENAME COLUMN extractor_url TO original_url;

--bun:split

ALTER TABLE tracks RENAME COLUMN stream_url TO url;
