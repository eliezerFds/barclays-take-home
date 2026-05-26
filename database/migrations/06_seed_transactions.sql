-- +migrate Up

INSERT INTO transactions (id, account_number, amount, currency, type, reference, created_at)
VALUES (
    'tan-seed00000000000000000000000001',
    '01000001',
    10000,
    'GBP',
    'deposit',
    'Opening balance',
    '2024-01-01T00:00:00Z'
);

-- +migrate Down

DELETE FROM transactions WHERE id = 'tan-seed00000000000000000000000001';
