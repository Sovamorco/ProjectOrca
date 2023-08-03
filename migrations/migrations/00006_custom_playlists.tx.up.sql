-- noinspection SqlResolveForFile
CREATE TABLE playlists
(
    id      VARCHAR(36)  NOT NULL PRIMARY KEY,
    user_id VARCHAR(128) NOT NULL,
    name    TEXT         NOT NULL
);

CREATE INDEX playlists_user_id ON playlists (user_id);

CREATE TABLE playlist_tracks
(
    id             VARCHAR(36)      NOT NULL PRIMARY KEY,
    playlist_id    VARCHAR(36)      NOT NULL,
    duration       BIGINT           NOT NULL,
    ord_key        DOUBLE PRECISION NOT NULL,
    title          TEXT             NOT NULL,
    extraction_url TEXT             NOT NULL,
    stream_url     TEXT             NOT NULL,
    http_headers   JSON             NOT NULL,
    live           BOOL             NOT NULL,
    display_url    TEXT             NOT NULL
);

CREATE INDEX playlist_tracks_playlist_id_ord_key ON playlist_tracks (playlist_id, ord_key);
