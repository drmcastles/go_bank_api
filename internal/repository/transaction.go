package repository

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/google/uuid"
	"time"

	"github.com/sirupsen/logrus"

	"banking-api/internal/model"
)

type TransactionRepository struct {
	db     *sql.DB
	logger *logrus.Logger
}

func NewTransactionRepository(db *sql.DB, logger *logrus.Logger) *TransactionRepository {
	return &TransactionRepository{db: db, logger: logger}
}

func (r *TransactionRepository) CreateTx(ctx context.Context, tx *sql.Tx, transaction *model.Transaction) error {
	r.logger.WithFields(logrus.Fields{
		"transaction_id": transaction.ID,
		"account_id":     transaction.AccountID,
		"amount":         transaction.Amount,
		"type":           transaction.TransactionType,
		"reference_id":   transaction.ReferenceID,
		"created_at":     transaction.CreatedAt,
	}).Info("Создание новой транзакции")

	query := `
        INSERT INTO transactions (id, account_id, amount, transaction_type, reference_id, created_at)
        VALUES ($1, $2, $3, $4, $5, $6)
    `

	_, err := tx.ExecContext(
		ctx,
		query,
		transaction.ID,
		transaction.AccountID,
		transaction.Amount,
		transaction.TransactionType,
		transaction.ReferenceID,
		transaction.CreatedAt,
	)

	if err != nil {
		r.logger.WithError(err).Error("Ошибка при создании транзакции")
		return fmt.Errorf("failed to create transaction: %w", err)
	}

	r.logger.Info("Транзакция успешно создана")
	return nil
}

// GetByAccountAndPeriod возвращает транзакции по счету за период
func (r *TransactionRepository) GetByAccountAndPeriod(
	ctx context.Context,
	accountID uuid.UUID,
	startDate, endDate time.Time,
) ([]model.Transaction, error) {
	// Добавляем 1 день к endDate, чтобы включить весь последний день периода
	endDate = endDate.Add(24 * time.Hour)

	r.logger.WithFields(logrus.Fields{
		"account_id": accountID,
		"start_date": startDate.Format("2006-01-02"),
		"end_date":   endDate.Format("2006-01-02"),
	}).Debug("Запрос транзакций по счету за период")

	const query = `SELECT id, account_id, amount, transaction_type, reference_id, created_at 
                  FROM transactions 
                  WHERE account_id = $1 AND created_at >= $2 AND created_at < $3
                  ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query, accountID, startDate, endDate)
	if err != nil {
		r.logger.WithFields(logrus.Fields{
			"error":      err.Error(),
			"account_id": accountID,
		}).Error("Ошибка запроса транзакций")
		return nil, fmt.Errorf("ошибка получения транзакций: %w", err)
	}
	defer rows.Close()

	var transactions []model.Transaction
	for rows.Next() {
		var tx model.Transaction
		if err := rows.Scan(
			&tx.ID,
			&tx.AccountID,
			&tx.Amount,
			&tx.TransactionType,
			&tx.ReferenceID,
			&tx.CreatedAt,
		); err != nil {
			r.logger.WithError(err).Error("Ошибка чтения строки транзакции")
			return nil, fmt.Errorf("ошибка чтения транзакции: %w", err)
		}
		transactions = append(transactions, tx)
	}

	if err := rows.Err(); err != nil {
		r.logger.WithError(err).Error("Ошибка при обработке результатов")
		return nil, fmt.Errorf("ошибка обработки результатов: %w", err)
	}

	r.logger.WithField("count", len(transactions)).Debug("Транзакции успешно получены")
	return transactions, nil
}
