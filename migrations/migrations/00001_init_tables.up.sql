CREATE TABLE IF NOT EXISTS states
(
    id          VARCHAR(128) NOT NULL PRIMARY KEY,
    state_token VARCHAR(512) NOT NULL UNIQUE,
    token       VARCHAR(512) NOT NULL UNIQUE
);

--bun:split

CREATE TABLE IF NOT EXISTS guild_states
(
    id            VARCHAR(128) NOT NULL PRIMARY KEY,
    bot_id        VARCHAR(128) NOT NULL,
    volume        DOUBLE       NOT NULL,
    target_volume DOUBLE       NOT NULL,

    FOREIGN KEY (bot_id) REFERENCES states(id)
);

--bun:split

CREATE TABLE IF NOT EXISTS queues
(
    id         VARCHAR(36) NOT NULL PRIMARY KEY,
    guild_id   VARCHAR(128) NOT NULL,
    channel_id TEXT NULL,

    FOREIGN KEY (guild_id) REFERENCES guild_states (id)
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
