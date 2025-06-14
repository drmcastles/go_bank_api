CREATE TABLE payment_schedules
(
    id             UUID PRIMARY KEY,
    credit_id      UUID           NOT NULL REFERENCES credits (id),
    payment_number INTEGER        NOT NULL,
    payment_date   TIMESTAMP      NOT NULL,
    amount         DECIMAL(15, 2) NOT NULL,
    principal      DECIMAL(15, 2) NOT NULL,
    interest       DECIMAL(15, 2) NOT NULL,
    status         VARCHAR(20)    NOT NULL,
    paid_at        TIMESTAMP,
    created_at     TIMESTAMP      NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMP      NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_payment_schedules_credit_id ON payment_schedules (credit_id);
CREATE INDEX idx_payment_schedules_status ON payment_schedules (status);
CREATE INDEX idx_payment_schedules_payment_date ON payment_schedules (payment_date);