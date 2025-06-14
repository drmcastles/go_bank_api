CREATE TABLE credits
(
    id              UUID PRIMARY KEY,
    account_id      UUID           NOT NULL REFERENCES accounts (id),
    user_id         UUID           NOT NULL REFERENCES users (id),
    amount          DECIMAL(15, 2) NOT NULL,
    interest_rate   DECIMAL(5, 2)  NOT NULL,
    term_months     INTEGER        NOT NULL,
    monthly_payment DECIMAL(15, 2) NOT NULL,
    start_date      TIMESTAMP      NOT NULL,
    end_date        TIMESTAMP      NOT NULL,
    status          VARCHAR(20)    NOT NULL,
    created_at      TIMESTAMP      NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMP      NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_credits_user_id ON credits (user_id);
CREATE INDEX idx_credits_account_id ON credits (account_id);