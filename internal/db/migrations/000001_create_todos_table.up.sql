CREATE TABLE IF NOT EXISTS todos (
    id        TEXT PRIMARY KEY,
    title     TEXT        NOT NULL,
    completed BOOLEAN     NOT NULL DEFAULT FALSE
);