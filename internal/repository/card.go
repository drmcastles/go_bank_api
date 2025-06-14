package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"banking-api/internal/model"
)

type CardRepository struct {
	db     *sql.DB
	logger *logrus.Logger
}

func NewCardRepository(db *sql.DB, logger *logrus.Logger) *CardRepository {
	return &CardRepository{db: db, logger: logger}
}

func (r *CardRepository) Create(ctx context.Context, card *model.Card) error {
	query := `
        INSERT INTO cards (id, user_id, account_id, name, encrypted_data, cvv_hash, hmac, created_at, last_used_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
    `
	_, err := r.db.ExecContext(ctx, query,
		card.ID,
		card.UserID,
		card.AccountID,
		card.Name,
		card.EncryptedData,
		card.CVVHash,
		card.HMAC,
		card.CreatedAt,
		card.LastUsedAt,
	)
	return err
}

func (r *CardRepository) GetByIDAndUser(ctx context.Context, cardID, userID uuid.UUID) (*model.Card, error) {
	query := `
		SELECT id, user_id, account_id, encrypted_data, cvv_hash, hmac, created_at, last_used_at
		FROM cards
		WHERE id = $1 AND user_id = $2
	`
	var card model.Card
	err := r.db.QueryRowContext(ctx, query, cardID, userID).Scan(
		&card.ID,
		&card.UserID,
		&card.AccountID,
		&card.EncryptedData,
		&card.CVVHash,
		&card.HMAC,
		&card.CreatedAt,
		&card.LastUsedAt,
	)
	if err != nil {
		return nil, err
	}
	return &card, nil
}

func (r *CardRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]model.Card, error) {
	query := `
        SELECT id, user_id, account_id, encrypted_data, cvv_hash, hmac, created_at, last_used_at
        FROM cards
        WHERE user_id = $1
        ORDER BY created_at DESC
    `

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query user cards: %w", err)
	}
	defer rows.Close()

	var cards []model.Card
	for rows.Next() {
		var card model.Card
		if err := rows.Scan(
			&card.ID,
			&card.UserID,
			&card.AccountID,
			&card.EncryptedData,
			&card.CVVHash,
			&card.HMAC,
			&card.CreatedAt,
			&card.LastUsedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan card: %w", err)
		}
		cards = append(cards, card)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return cards, nil
}

func (r *CardRepository) UpdateLastUsed(ctx context.Context, cardID uuid.UUID) error {
	query := `
		UPDATE cards
		SET last_used_at = $1
		WHERE id = $2
	`
	_, err := r.db.ExecContext(ctx, query, time.Now(), cardID)
	return err
}
