package model

import (
	"time"

	"github.com/google/uuid"
)

type TransactionType string

const (
	TransactionTypeTransfer      TransactionType = "transfer"       // перевод между счетами
	TransactionTypeDeposit       TransactionType = "deposit"        // пополнение счета
	TransactionTypeWithdrawal    TransactionType = "withdrawal"     // вывод средств со счета
	TransactionTypeCredit        TransactionType = "credit"         // выдача кредита
	TransactionTypeCreditPayment TransactionType = "credit_payment" // платеж по кредиту
	TransactionTypeCardPayment   TransactionType = "card_payment"   // платеж картой
)

type Transaction struct {
	ID              uuid.UUID       `json:"id" db:"id"`
	AccountID       uuid.UUID       `json:"account_id" db:"account_id"`
	Amount          float64         `json:"amount" db:"amount"`
	TransactionType TransactionType `json:"transaction_type" db:"transaction_type"`
	ReferenceID     *uuid.UUID      `json:"reference_id" db:"reference_id"`
	CreatedAt       time.Time       `json:"created_at" db:"created_at"`
}
