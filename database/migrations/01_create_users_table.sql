-- +migrate Up

CREATE TABLE users (
    id               TEXT PRIMARY KEY,
    name             TEXT NOT NULL,
    address_line1    TEXT NOT NULL,
    address_line2    TEXT,
    address_line3    TEXT,
    address_town     TEXT NOT NULL,
    address_county   TEXT NOT NULL,
    address_postcode TEXT NOT NULL,
    phone_number     TEXT NOT NULL,
    email            TEXT NOT NULL,
    password_hash    TEXT NOT NULL,
    created_at       TIMESTAMP NOT NULL,
    updated_at       TIMESTAMP NOT NULL,
    UNIQUE (email)
);

-- +migrate Down

DROP TABLE users;
