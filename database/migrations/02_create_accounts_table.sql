-- +migrate Up

CREATE TABLE accounts (
    account_number   TEXT PRIMARY KEY,
    sort_code        TEXT NOT NULL DEFAULT '10-10-10',
    name             TEXT NOT NULL,
    account_type     TEXT NOT NULL DEFAULT 'personal',
    balance          REAL NOT NULL DEFAULT 0.00,
    currency         TEXT NOT NULL DEFAULT 'GBP',
    user_id          TEXT NOT NULL,
    created_at       TIMESTAMP NOT NULL,
    updated_at       TIMESTAMP NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users(id)
);

-- +migrate Down

DROP TABLE accounts;
