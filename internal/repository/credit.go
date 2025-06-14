package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/sirupsen/logrus"

	"banking-api/internal/model"
)

type CreditRepository struct {
	db     *sql.DB
	logger *logrus.Logger
}

func NewCreditRepository(db *sql.DB, logger *logrus.Logger) *CreditRepository {
	return &CreditRepository{db: db, logger: logger}
}

func (r *CreditRepository) CreateCredit(ctx context.Context, credit *model.Credit) error {
	query := `
        INSERT INTO credits (id, account_id, user_id, amount, interest_rate, term_months, 
                            monthly_payment, start_date, end_date, status, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
    `

	_, err := r.db.ExecContext(
		ctx,
		query,
		credit.ID,
		credit.AccountID,
		credit.UserID,
		credit.Amount,
		credit.InterestRate,
		credit.TermMonths,
		credit.MonthlyPayment,
		credit.StartDate,
		credit.EndDate,
		credit.Status,
		credit.CreatedAt,
		credit.UpdatedAt,
	)

	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok {
			if pqErr.Code.Name() == "foreign_key_violation" {
				return fmt.Errorf("account not found")
			}
		}
		return fmt.Errorf("failed to create credit: %w", err)
	}

	return nil
}

func (r *CreditRepository) GetCreditByID(ctx context.Context, id uuid.UUID) (*model.Credit, error) {
	query := `
        SELECT id, account_id, user_id, amount, interest_rate, term_months, 
               monthly_payment, start_date, end_date, status, created_at, updated_at
        FROM credits
        WHERE id = $1
    `

	var credit model.Credit
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&credit.ID,
		&credit.AccountID,
		&credit.UserID,
		&credit.Amount,
		&credit.InterestRate,
		&credit.TermMonths,
		&credit.MonthlyPayment,
		&credit.StartDate,
		&credit.EndDate,
		&credit.Status,
		&credit.CreatedAt,
		&credit.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("credit not found")
		}
		return nil, fmt.Errorf("failed to get credit: %w", err)
	}

	return &credit, nil
}

func (r *CreditRepository) GetUserCredits(ctx context.Context, userID uuid.UUID) ([]model.Credit, error) {
	query := `
        SELECT id, account_id, user_id, amount, interest_rate, term_months, 
               monthly_payment, start_date, end_date, status, created_at, updated_at
        FROM credits
        WHERE user_id = $1
    `

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query user credits: %w", err)
	}
	defer rows.Close()

	var credits []model.Credit
	for rows.Next() {
		var credit model.Credit
		if err := rows.Scan(
			&credit.ID,
			&credit.AccountID,
			&credit.UserID,
			&credit.Amount,
			&credit.InterestRate,
			&credit.TermMonths,
			&credit.MonthlyPayment,
			&credit.StartDate,
			&credit.EndDate,
			&credit.Status,
			&credit.CreatedAt,
			&credit.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan credit: %w", err)
		}
		credits = append(credits, credit)
	}

	return credits, nil
}

func (r *CreditRepository) CreatePaymentSchedule(ctx context.Context, schedule *model.PaymentSchedule) error {
	query := `
        INSERT INTO payment_schedules (id, credit_id, payment_number, payment_date, 
                                     amount, principal, interest, status, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
    `

	_, err := r.db.ExecContext(
		ctx,
		query,
		schedule.ID,
		schedule.CreditID,
		schedule.PaymentNumber,
		schedule.PaymentDate,
		schedule.Amount,
		schedule.Principal,
		schedule.Interest,
		schedule.Status,
		schedule.CreatedAt,
		schedule.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create payment schedule: %w", err)
	}

	return nil
}

func (r *CreditRepository) GetPaymentSchedule(ctx context.Context, creditID uuid.UUID) ([]model.PaymentSchedule, error) {
	query := `
        SELECT id, credit_id, payment_number, payment_date, amount, 
               principal, interest, status, paid_at, created_at, updated_at
        FROM payment_schedules
        WHERE credit_id = $1
        ORDER BY payment_number
    `

	rows, err := r.db.QueryContext(ctx, query, creditID)
	if err != nil {
		return nil, fmt.Errorf("failed to query payment schedule: %w", err)
	}
	defer rows.Close()

	var schedules []model.PaymentSchedule
	for rows.Next() {
		var schedule model.PaymentSchedule
		if err := rows.Scan(
			&schedule.ID,
			&schedule.CreditID,
			&schedule.PaymentNumber,
			&schedule.PaymentDate,
			&schedule.Amount,
			&schedule.Principal,
			&schedule.Interest,
			&schedule.Status,
			&schedule.PaidAt,
			&schedule.CreatedAt,
			&schedule.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan payment schedule: %w", err)
		}
		schedules = append(schedules, schedule)
	}

	return schedules, nil
}

func (r *CreditRepository) GetPendingPayments(ctx context.Context, before time.Time) ([]model.PaymentSchedule, error) {
	query := `
        SELECT id, credit_id, payment_number, payment_date, amount, 
               principal, interest, status, paid_at, created_at, updated_at
        FROM payment_schedules
        WHERE status = 'pending' AND payment_date <= $1
        ORDER BY payment_date
    `

	rows, err := r.db.QueryContext(ctx, query, before)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending payments: %w", err)
	}
	defer rows.Close()

	var payments []model.PaymentSchedule
	for rows.Next() {
		var payment model.PaymentSchedule
		if err := rows.Scan(
			&payment.ID,
			&payment.CreditID,
			&payment.PaymentNumber,
			&payment.PaymentDate,
			&payment.Amount,
			&payment.Principal,
			&payment.Interest,
			&payment.Status,
			&payment.PaidAt,
			&payment.CreatedAt,
			&payment.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan payment: %w", err)
		}
		payments = append(payments, payment)
	}

	return payments, nil
}

func (r *CreditRepository) UpdatePaymentStatus(ctx context.Context, paymentID uuid.UUID, status string, paidAt *time.Time) error {
	query := `
        UPDATE payment_schedules
        SET status = $1,
            paid_at = $2,
            updated_at = NOW()
        WHERE id = $3
    `

	_, err := r.db.ExecContext(ctx, query, status, paidAt, paymentID)
	if err != nil {
		return fmt.Errorf("failed to update payment status: %w", err)
	}

	return nil
}

func (r *CreditRepository) UpdateCreditStatus(ctx context.Context, creditID uuid.UUID, status string) error {
	query := `
        UPDATE credits
        SET status = $1,
            updated_at = NOW()
        WHERE id = $2
    `

	_, err := r.db.ExecContext(ctx, query, status, creditID)
	if err != nil {
		return fmt.Errorf("failed to update credit status: %w", err)
	}

	return nil
}

func (r *CreditRepository) GetDB() *sql.DB {
	return r.db
}

func (r *CreditRepository) GetPaymentByID(ctx context.Context, id uuid.UUID) (*model.PaymentSchedule, error) {
	query := `
        SELECT id, credit_id, payment_number, payment_date, amount, 
               principal, interest, status, paid_at, created_at, updated_at
        FROM payment_schedules
        WHERE id = $1
    `

	var payment model.PaymentSchedule
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&payment.ID,
		&payment.CreditID,
		&payment.PaymentNumber,
		&payment.PaymentDate,
		&payment.Amount,
		&payment.Principal,
		&payment.Interest,
		&payment.Status,
		&payment.PaidAt,
		&payment.CreatedAt,
		&payment.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("платеж не найден: %w", err)
	}

	return &payment, nil
}
