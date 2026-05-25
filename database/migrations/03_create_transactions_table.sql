-- +migrate Up

CREATE TABLE transactions (
    id              TEXT PRIMARY KEY,
    account_number  TEXT NOT NULL,
    amount          INTEGER NOT NULL CHECK (amount > 0),
    currency        TEXT NOT NULL DEFAULT 'GBP',
    type            TEXT NOT NULL,
    reference       TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMP NOT NULL,
    FOREIGN KEY (account_number) REFERENCES accounts(account_number)
);

-- +migrate Down

DROP TABLE transactions;
