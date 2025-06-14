CREATE TABLE cards
(
    id             UUID PRIMARY KEY                  DEFAULT gen_random_uuid(),
    user_id        UUID                     NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    account_id     UUID                     NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    name           TEXT                     NOT NULL,
    encrypted_data TEXT                     NOT NULL, -- PGP-encrypted (number|expiry)
    cvv_hash       TEXT                     NOT NULL, -- bcrypt hash
    hmac           TEXT                     NOT NULL, -- HMAC-SHA256
    created_at     TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    last_used_at   TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_cards_user_id ON cards (user_id);
CREATE INDEX idx_cards_account_id ON cards (account_id);