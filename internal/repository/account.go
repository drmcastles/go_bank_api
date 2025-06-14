package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/sirupsen/logrus"

	"banking-api/internal/model"
)

type AccountRepository struct {
	db     *sql.DB
	logger *logrus.Logger
}

func NewAccountRepository(db *sql.DB, logger *logrus.Logger) *AccountRepository {
	return &AccountRepository{db: db, logger: logger}
}

func (r *AccountRepository) Create(ctx context.Context, account *model.Account) error {
	query := `
		INSERT INTO accounts (id, user_id, balance, currency, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	_, err := r.db.ExecContext(
		ctx,
		query,
		account.ID,
		account.UserID,
		account.Balance,
		account.Currency,
		account.CreatedAt,
		account.UpdatedAt,
	)

	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok {
			if pqErr.Code.Name() == "unique_violation" {
				return fmt.Errorf("account already exists")
			}
		}
		return fmt.Errorf("failed to create account: %w", err)
	}

	return nil
}

func (r *AccountRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Account, error) {
	query := `
        SELECT id, user_id, balance, currency, created_at, updated_at
        FROM accounts
        WHERE id = $1
    `

	var account model.Account
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&account.ID,
		&account.UserID,
		&account.Balance,
		&account.Currency,
		&account.CreatedAt,
		&account.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("account not found")
		}
		return nil, fmt.Errorf("failed to get account: %w", err)
	}

	return &account, nil
}

func (r *AccountRepository) GetByIDForUpdate(ctx context.Context, tx *sql.Tx, id uuid.UUID) (*model.Account, error) {
	query := `
        SELECT id, user_id, balance, currency, created_at, updated_at
        FROM accounts
        WHERE id = $1
        FOR UPDATE
    `

	var account model.Account
	err := tx.QueryRowContext(ctx, query, id).Scan(
		&account.ID,
		&account.UserID,
		&account.Balance,
		&account.Currency,
		&account.CreatedAt,
		&account.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get account: %w", err)
	}
	return &account, nil
}

func (r *AccountRepository) UpdateBalanceTx(ctx context.Context, tx *sql.Tx, id uuid.UUID, amount float64) error {
	query := `
        UPDATE accounts
        SET balance = balance + $1,
            updated_at = NOW()
        WHERE id = $2
    `

	result, err := tx.ExecContext(ctx, query, amount, id)
	if err != nil {
		return fmt.Errorf("failed to update balance: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("account not found")
	}

	return nil
}

func (r *AccountRepository) GetDB() *sql.DB {
	return r.db
}

func (r *AccountRepository) GetUserAccounts(ctx context.Context, userID uuid.UUID) ([]model.Account, error) {
	query := `
		SELECT id, user_id, balance, currency, created_at, updated_at
		FROM accounts
		WHERE user_id = $1
	`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query user accounts: %w", err)
	}
	defer rows.Close()

	var accounts []model.Account
	for rows.Next() {
		var account model.Account
		if err := rows.Scan(
			&account.ID,
			&account.UserID,
			&account.Balance,
			&account.Currency,
			&account.CreatedAt,
			&account.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan account: %w", err)
		}
		accounts = append(accounts, account)
	}

	return accounts, nil
}
