CREATE TABLE accounts
(
    id         UUID PRIMARY KEY                  DEFAULT gen_random_uuid(),
    user_id    UUID                     NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    balance    DECIMAL(15, 2)           NOT NULL DEFAULT 0,
    currency   VARCHAR(3)               NOT NULL DEFAULT 'RUB',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (balance >= 0)
);

CREATE INDEX idx_accounts_user_id ON accounts (user_id);

CREATE TABLE transactions
(
    id               UUID PRIMARY KEY                  DEFAULT gen_random_uuid(),
    account_id       UUID                     NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    amount           DECIMAL(15, 2)           NOT NULL,
    transaction_type VARCHAR(20)              NOT NULL,
    reference_id     UUID,
    created_at       TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (amount > 0)
);

CREATE INDEX idx_transactions_account_id ON transactions (account_id);
CREATE INDEX idx_transactions_reference_id ON transactions (reference_id);