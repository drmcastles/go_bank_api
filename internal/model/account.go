package model

import (
	"time"

	"github.com/google/uuid"
)

type Account struct {
	ID        uuid.UUID `json:"id" db:"id"`
	UserID    uuid.UUID `json:"user_id" db:"user_id"`
	Balance   float64   `json:"balance" db:"balance"`
	Currency  string    `json:"currency" db:"currency"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

type CreateAccountRequest struct {
	Currency string `json:"currency" validate:"required,oneof=RUB"`
}

type TransferRequest struct {
	FromAccountID uuid.UUID `json:"from_account_id" validate:"required"`
	ToAccountID   uuid.UUID `json:"to_account_id" validate:"required"`
	Amount        float64   `json:"amount" validate:"required,gt=0"`
}

type ChangeRequest struct {
	AccountID uuid.UUID `json:"account_id" validate:"required"`
	Amount    float64   `json:"amount" validate:"required,gt=0"`
}
