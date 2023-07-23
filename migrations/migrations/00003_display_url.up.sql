# noinspection SqlResolveForFile

ALTER TABLE tracks ADD COLUMN display_url TEXT NOT NULL;

--bun:split

ALTER TABLE tracks RENAME COLUMN original_url TO extractor_url;

--bun:split

ALTER TABLE tracks RENAME COLUMN url TO stream_url;

--bun:split

# noinspection SqlWithoutWhere
UPDATE tracks SET display_url=extractor_url;
