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