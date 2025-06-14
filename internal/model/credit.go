package model

import (
	"time"

	"github.com/google/uuid"
)

// Withdraw models
type Credit struct {
	ID             uuid.UUID `json:"id" db:"id"`
	AccountID      uuid.UUID `json:"account_id" db:"account_id"`
	UserID         uuid.UUID `json:"user_id" db:"user_id"`
	Amount         float64   `json:"amount" db:"amount"`
	InterestRate   float64   `json:"interest_rate" db:"interest_rate"`
	TermMonths     int       `json:"term_months" db:"term_months"`
	MonthlyPayment float64   `json:"monthly_payment" db:"monthly_payment"`
	StartDate      time.Time `json:"start_date" db:"start_date"`
	EndDate        time.Time `json:"end_date" db:"end_date"`
	Status         string    `json:"status" db:"status"` // active, paid, overdue, defaulted
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}

type PaymentSchedule struct {
	ID            uuid.UUID  `json:"id" db:"id"`
	CreditID      uuid.UUID  `json:"credit_id" db:"credit_id"`
	PaymentNumber int        `json:"payment_number" db:"payment_number"`
	PaymentDate   time.Time  `json:"payment_date" db:"payment_date"`
	Amount        float64    `json:"amount" db:"amount"`
	Principal     float64    `json:"principal" db:"principal"`
	Interest      float64    `json:"interest" db:"interest"`
	Status        string     `json:"status" db:"status"` // pending, paid, overdue
	PaidAt        *time.Time `json:"paid_at" db:"paid_at"`
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at" db:"updated_at"`
}

type CreateCreditRequest struct {
	AccountID  uuid.UUID `json:"account_id" validate:"required"`
	Amount     float64   `json:"amount" validate:"required,gt=0"`
	TermMonths int       `json:"term_months" validate:"required,gte=6,lte=60"`
}

type CreditPaymentRequest struct {
	CreditID uuid.UUID `json:"credit_id" validate:"required"`
	Amount   float64   `json:"amount" validate:"required,gt=0"`
}
