CREATE TABLE instance_settings (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    token_secret TEXT NOT NULL DEFAULT ''
);

INSERT INTO instance_settings (id) VALUES (1);

CREATE TABLE users (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    username TEXT NOT NULL UNIQUE CHECK (username <> ''),
    password_hash TEXT NOT NULL CHECK (password_hash <> '')
);
