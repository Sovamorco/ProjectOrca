CREATE TABLE IF NOT EXISTS bots
(
    id          VARCHAR(128) NOT NULL PRIMARY KEY,
    token       VARCHAR(512) NOT NULL UNIQUE,
    locker      VARCHAR(36)  NULL
);

--bun:split

CREATE TABLE IF NOT EXISTS guilds
(
    bot_id     VARCHAR(128) NOT NULL,
    id         VARCHAR(128) NOT NULL,
    channel_id VARCHAR(128) NULL,
    paused     BOOL         NOT NULL,
    `loop`     BOOL         NOT NULL,

    PRIMARY KEY (bot_id, id)
);

--bun:split

CREATE TABLE IF NOT EXISTS tracks
(
    id           VARCHAR(36)  NOT NULL PRIMARY KEY,
    bot_id       VARCHAR(128) NOT NULL,
    guild_id     VARCHAR(128) NOT NULL,
    pos          BIGINT       NOT NULL,
    duration     BIGINT       NOT NULL,
    ord_key      DOUBLE       NOT NULL,
    title        TEXT         NOT NULL,
    original_url TEXT         NOT NULL,
    url          TEXT         NOT NULL,
    http_headers JSON         NOT NULL,
    live         BOOL         NOT NULL
);
