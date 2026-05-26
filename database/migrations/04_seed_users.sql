-- +migrate Up

INSERT INTO users (id, name, address_line1, address_line2, address_line3, address_town, address_county, address_postcode, phone_number, email, password_hash, created_at, updated_at)
VALUES (
    'usr-seed00000000000000000000000001',
    'Seed User',
    '1 Seed Street',
    NULL,
    NULL,
    'London',
    'Greater London',
    'EC1A 1BB',
    '+447700900000',
    'seed@example.com',
    '$2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi',
    '2024-01-01T00:00:00Z',
    '2024-01-01T00:00:00Z'
);

-- +migrate Down

DELETE FROM users WHERE id = 'usr-seed00000000000000000000000001';
