package model

import (
	"time"

	"github.com/google/uuid"
)

type Card struct {
	ID            uuid.UUID `json:"id" db:"id"`
	UserID        uuid.UUID `json:"user_id" db:"user_id"`
	AccountID     uuid.UUID `json:"account_id" db:"account_id"`
	EncryptedData string    `json:"-" db:"encrypted_data"` // PGP-encrypted (number+expiry)
	CVVHash       string    `json:"-" db:"cvv_hash"`       // bcrypt hash
	HMAC          string    `json:"-" db:"hmac"`           // HMAC-SHA256
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	LastUsedAt    time.Time `json:"last_used_at" db:"last_used_at"`
	Name          string    `json:"name" db:"name"`
}

type CardRequest struct {
	AccountID uuid.UUID `json:"account_id" validate:"required"`
	Name      string    `json:"name" validate:"required"` // Для привязки карты
}

type CardResponse struct {
	ID           uuid.UUID `json:"id"`
	MaskedNumber string    `json:"masked_number"`
	Expiry       string    `json:"expiry"`
	Name         string    `json:"name"`
}

type PaymentRequest struct {
	CardID uuid.UUID `json:"card_id" validate:"required"`
	Amount float64   `json:"amount" validate:"required,gt=0"`
}

type PaymentResponse struct {
	PaymentID   uuid.UUID `json:"payment_id"`
	CardID      uuid.UUID `json:"card_id"`
	AccountID   uuid.UUID `json:"account_id"`
	Amount      float64   `json:"amount"`
	Status      string    `json:"status"` // pending, completed, failed
	ProcessedAt time.Time `json:"processed_at"`
}

type CardData struct {
	Number string `json:"number"`
	Expiry string `json:"expiry"`
}
