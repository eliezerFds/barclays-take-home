-- +migrate Up

INSERT INTO accounts (account_number, name, account_type, balance, user_id, created_at, updated_at)
VALUES (
    '01000001',
    'Seed Account',
    'personal',
    10000,
    'usr-seed00000000000000000000000001',
    '2024-01-01T00:00:00Z',
    '2024-01-01T00:00:00Z'
);

-- +migrate Down

DELETE FROM accounts WHERE account_number = '01000001';
