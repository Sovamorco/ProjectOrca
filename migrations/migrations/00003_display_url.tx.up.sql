-- noinspection SqlWithoutWhereForFile, SqlResolveForFile, SqlAddNotNullColumnForFile

ALTER TABLE tracks
    ADD COLUMN display_url TEXT NOT NULL;

ALTER TABLE tracks
    RENAME COLUMN original_url TO extractor_url;

ALTER TABLE tracks
    RENAME COLUMN url TO stream_url;

UPDATE tracks
SET display_url=extractor_url;
