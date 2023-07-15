CREATE TABLE IF NOT EXISTS bots
(
    id          VARCHAR(128) NOT NULL PRIMARY KEY,
    state_token VARCHAR(512) NOT NULL UNIQUE,
    token       VARCHAR(512) NOT NULL UNIQUE,
    locker      VARCHAR(36)  NULL
);

--bun:split

CREATE TABLE IF NOT EXISTS guilds
(
    id            VARCHAR(36)  NOT NULL PRIMARY KEY,
    guild_id      VARCHAR(128) NOT NULL,
    bot_id        VARCHAR(128) NOT NULL,
    volume        DOUBLE       NOT NULL,
    target_volume DOUBLE       NOT NULL,

    UNIQUE (bot_id, guild_id),
    FOREIGN KEY (bot_id) REFERENCES bots(id)
);

--bun:split

CREATE TABLE IF NOT EXISTS queues
(
    id         VARCHAR(36)  NOT NULL PRIMARY KEY,
    guild_id   VARCHAR(36)  NOT NULL,
    channel_id VARCHAR(128) NULL,

    FOREIGN KEY (guild_id) REFERENCES guilds(id)
);

--bun:split

CREATE TABLE IF NOT EXISTS tracks
(
    id         VARCHAR(36) NOT NULL PRIMARY KEY,
    queue_id   VARCHAR(36) NOT NULL,
    pos        BIGINT      NOT NULL,
    track_data JSON        NOT NULL,
    ord_key    DOUBLE      NULL,

    FOREIGN KEY (queue_id) REFERENCES queues(id)
);
