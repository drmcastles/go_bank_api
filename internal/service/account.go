package service

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"banking-api/internal/model"
	"banking-api/internal/repository"
)

type AccountService struct {
	userRepo        *repository.UserRepository
	accountRepo     *repository.AccountRepository
	transactionRepo *repository.TransactionRepository
	emailSender     *EmailSender
	logger          *logrus.Logger
}

type TransactionRepository struct {
	db     *sql.DB
	logger *logrus.Logger
}

func NewAccountService(
	userRepo *repository.UserRepository,
	accountRepo *repository.AccountRepository,
	transactionRepo *repository.TransactionRepository,
	emailSender *EmailSender,
	logger *logrus.Logger,
) *AccountService {
	return &AccountService{
		userRepo:        userRepo,
		accountRepo:     accountRepo,
		transactionRepo: transactionRepo,
		emailSender:     emailSender,
		logger:          logger,
	}
}

func (s *AccountService) CreateAccount(ctx context.Context, userID uuid.UUID, currency string) (*model.Account, error) {
	if currency != "RUB" {
		s.logger.Warnf("Попытка создания счета с валютой %s, поддерживается только RUB", currency)
		return nil, fmt.Errorf("поддерживается только валюта RUB")
	}

	now := time.Now()
	account := &model.Account{
		ID:        uuid.New(),
		UserID:    userID,
		Balance:   0,
		Currency:  currency,
		CreatedAt: now,
		UpdatedAt: now,
	}

	s.logger.Infof("Создание нового счета для пользователя %s", userID)
	if err := s.accountRepo.Create(ctx, account); err != nil {
		s.logger.WithError(err).Error("Ошибка при создании счета")
		return nil, fmt.Errorf("ошибка создания счета: %w", err)
	}

	s.logger.Infof("Успешно создан счет %s для пользователя %s", account.ID, userID)
	return account, nil
}

func (s *AccountService) GetUserAccounts(ctx context.Context, userID uuid.UUID) ([]model.Account, error) {
	s.logger.Infof("Получение списка счетов пользователя %s", userID)
	accounts, err := s.accountRepo.GetUserAccounts(ctx, userID)
	if err != nil {
		s.logger.WithError(err).Error("Ошибка при получении счетов пользователя")
		return nil, fmt.Errorf("ошибка получения счетов: %w", err)
	}
	return accounts, nil
}

func (s *AccountService) Transfer(
	ctx context.Context,
	fromAccountID uuid.UUID,
	toAccountID uuid.UUID,
	amount float64,
	userID uuid.UUID,
) error {
	if amount <= 0 {
		s.logger.Warn("Попытка перевода неположительной суммы")
		return fmt.Errorf("сумма перевода должна быть положительной")
	}

	s.logger.Infof("Инициирован перевод %.2f с счета %s на счет %s", amount, fromAccountID, toAccountID)

	// Получаем исходный счет и проверяем владельца
	fromAccount, err := s.accountRepo.GetByID(ctx, fromAccountID)
	if err != nil {
		s.logger.WithError(err).Errorf("Ошибка получения исходного счета %s", fromAccountID)
		return fmt.Errorf("ошибка получения счета отправителя: %w", err)
	}

	if fromAccount.UserID != userID {
		s.logger.Warnf("Попытка перевода с чужого счета: пользователь %s, владелец счета %s", userID, fromAccount.UserID)
		return fmt.Errorf("недостаточно прав: счет не принадлежит пользователю")
	}

	// Проверяем достаточность средств
	if fromAccount.Balance < amount {
		s.logger.Warnf("Недостаточно средств на счете %s: баланс %.2f, требуется %.2f",
			fromAccountID, fromAccount.Balance, amount)
		return fmt.Errorf("недостаточно средств на счете")
	}

	// Получаем целевой счет
	toAccount, err := s.accountRepo.GetByID(ctx, toAccountID)
	if err != nil {
		s.logger.WithError(err).Errorf("Ошибка получения целевого счета %s", toAccountID)
		return fmt.Errorf("ошибка получения счета получателя: %w", err)
	}

	// Проверяем валюту (только RUB)
	if fromAccount.Currency != "RUB" || toAccount.Currency != "RUB" {
		s.logger.Warnf("Попытка перевода между счетами с разными валютами: %s -> %s",
			fromAccount.Currency, toAccount.Currency)
		return fmt.Errorf("поддерживаются только переводы в RUB")
	}

	// Начинаем транзакцию
	db := s.accountRepo.GetDB()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		s.logger.WithError(err).Error("Ошибка начала транзакции")
		return fmt.Errorf("ошибка начала транзакции: %w", err)
	}
	defer tx.Rollback()

	// Списание со счета отправителя
	if err := s.accountRepo.UpdateBalanceTx(ctx, tx, fromAccountID, -amount); err != nil {
		s.logger.WithError(err).Errorf("Ошибка списания со счета %s", fromAccountID)
		return fmt.Errorf("ошибка списания средств: %w", err)
	}

	// Зачисление на счет получателя
	if err := s.accountRepo.UpdateBalanceTx(ctx, tx, toAccountID, amount); err != nil {
		s.logger.WithError(err).Errorf("Ошибка зачисления на счет %s", toAccountID)
		return fmt.Errorf("ошибка зачисления средств: %w", err)
	}

	// Создаем записи о транзакциях
	transferID := uuid.New()
	now := time.Now()

	debitTransaction := &model.Transaction{
		ID:              uuid.New(),
		AccountID:       fromAccountID,
		Amount:          amount,
		TransactionType: model.TransactionTypeTransfer,
		ReferenceID:     &transferID,
		CreatedAt:       now,
	}

	creditTransaction := &model.Transaction{
		ID:              uuid.New(),
		AccountID:       toAccountID,
		Amount:          amount,
		TransactionType: model.TransactionTypeTransfer,
		ReferenceID:     &transferID,
		CreatedAt:       now,
	}

	if err := s.transactionRepo.CreateTx(ctx, tx, debitTransaction); err != nil {
		s.logger.WithError(err).Error("Ошибка создания записи о списании")
		return fmt.Errorf("ошибка записи транзакции списания: %w", err)
	}

	if err := s.transactionRepo.CreateTx(ctx, tx, creditTransaction); err != nil {
		s.logger.WithError(err).Error("Ошибка создания записи о зачислении")
		return fmt.Errorf("ошибка записи транзакции зачисления: %w", err)
	}

	// Коммитим транзакцию
	if err := tx.Commit(); err != nil {
		s.logger.WithError(err).Error("Ошибка подтверждения транзакции")
		return fmt.Errorf("ошибка подтверждения перевода: %w", err)
	}

	s.logger.Infof("Успешно выполнен перевод %.2f с счета %s на счет %s", amount, fromAccountID, toAccountID)

	// После успешного перевода
	user, err := s.userRepo.GetByID(ctx, userID)
	if err == nil && user.Email != "" {
		go func() {
			if err := s.emailSender.SendTransferNotification(
				user.Email,
				amount,
				fromAccountID.String(),
				toAccountID.String(),
			); err != nil {
				s.logger.WithError(err).Warn("Не удалось отправить email уведомление")
			}
		}()
	}
	return nil
}

func (s *AccountService) Deposit(
	ctx context.Context,
	accountID uuid.UUID,
	amount float64,
	userID uuid.UUID,
) error {
	if amount <= 0 {
		s.logger.Warn("Попытка пополнения на неположительную сумму")
		return fmt.Errorf("сумма пополнения должна быть положительной")
	}

	s.logger.Infof("Инициировано пополнение счета %s на сумму %.2f", accountID, amount)

	// Получаем счет и проверяем владельца
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		s.logger.WithError(err).Errorf("Ошибка получения счета %s", accountID)
		return fmt.Errorf("ошибка получения счета: %w", err)
	}

	if account.UserID != userID {
		s.logger.Warnf("Попытка пополнения чужого счета: пользователь %s, владелец %s", userID, account.UserID)
		return fmt.Errorf("недостаточно прав: счет не принадлежит пользователю")
	}

	// Проверяем валюту (только RUB)
	if account.Currency != "RUB" {
		s.logger.Warnf("Попытка пополнения счета с валютой %s", account.Currency)
		return fmt.Errorf("поддерживаются только счета в RUB")
	}

	// Начинаем транзакцию
	db := s.accountRepo.GetDB()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		s.logger.WithError(err).Error("Ошибка начала транзакции")
		return fmt.Errorf("ошибка начала транзакции: %w", err)
	}
	defer tx.Rollback()

	// Зачисление на счет
	if err := s.accountRepo.UpdateBalanceTx(ctx, tx, accountID, amount); err != nil {
		s.logger.WithError(err).Errorf("Ошибка зачисления на счет %s", accountID)
		return fmt.Errorf("ошибка пополнения счета: %w", err)
	}

	// Создаем запись о транзакции
	transferID := uuid.New()
	now := time.Now()

	transaction := &model.Transaction{
		ID:              uuid.New(),
		AccountID:       accountID,
		Amount:          amount,
		TransactionType: model.TransactionTypeDeposit,
		ReferenceID:     &transferID,
		CreatedAt:       now,
	}

	if err := s.transactionRepo.CreateTx(ctx, tx, transaction); err != nil {
		s.logger.WithError(err).Error("Ошибка создания записи о пополнении")
		return fmt.Errorf("ошибка записи транзакции: %w", err)
	}

	// Коммитим транзакцию
	if err := tx.Commit(); err != nil {
		s.logger.WithError(err).Error("Ошибка подтверждения транзакции")
		return fmt.Errorf("ошибка подтверждения операции: %w", err)
	}

	s.logger.Infof("Успешно пополнен счет %s на сумму %.2f", accountID, amount)
	return nil
}

func (s *AccountService) Withdraw(
	ctx context.Context,
	accountID uuid.UUID,
	amount float64,
	userID uuid.UUID,
) error {
	if amount <= 0 {
		s.logger.Warn("Попытка снятия неположительной суммы")
		return fmt.Errorf("сумма снятия должна быть положительной")
	}

	s.logger.Infof("Инициировано снятие со счета %s суммы %.2f", accountID, amount)

	// Получаем счет и проверяем владельца
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		s.logger.WithError(err).Errorf("Ошибка получения счета %s", accountID)
		return fmt.Errorf("ошибка получения счета: %w", err)
	}

	if account.UserID != userID {
		s.logger.Warnf("Попытка снятия с чужого счета: пользователь %s, владелец %s", userID, account.UserID)
		return fmt.Errorf("недостаточно прав: счет не принадлежит пользователю")
	}

	// Проверяем достаточность средств
	if account.Balance < amount {
		s.logger.Warnf("Недостаточно средств на счете %s: баланс %.2f, требуется %.2f",
			accountID, account.Balance, amount)
		return fmt.Errorf("недостаточно средств на счете")
	}

	// Проверяем валюту (только RUB)
	if account.Currency != "RUB" {
		s.logger.Warnf("Попытка снятия со счета с валютой %s", account.Currency)
		return fmt.Errorf("поддерживаются только счета в RUB")
	}

	// Начинаем транзакцию
	db := s.accountRepo.GetDB()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		s.logger.WithError(err).Error("Ошибка начала транзакции")
		return fmt.Errorf("ошибка начала транзакции: %w", err)
	}
	defer tx.Rollback()

	// Списание со счета
	if err := s.accountRepo.UpdateBalanceTx(ctx, tx, accountID, -amount); err != nil {
		s.logger.WithError(err).Errorf("Ошибка списания со счета %s", accountID)
		return fmt.Errorf("ошибка снятия средств: %w", err)
	}

	// Создаем запись о транзакции
	transferID := uuid.New()
	now := time.Now()

	transaction := &model.Transaction{
		ID:              uuid.New(),
		AccountID:       accountID,
		Amount:          amount,
		TransactionType: model.TransactionTypeWithdrawal,
		ReferenceID:     &transferID,
		CreatedAt:       now,
	}

	if err := s.transactionRepo.CreateTx(ctx, tx, transaction); err != nil {
		s.logger.WithError(err).Error("Ошибка создания записи о снятии")
		return fmt.Errorf("ошибка записи транзакции: %w", err)
	}

	// Коммитим транзакцию
	if err := tx.Commit(); err != nil {
		s.logger.WithError(err).Error("Ошибка подтверждения транзакции")
		return fmt.Errorf("ошибка подтверждения операции: %w", err)
	}

	s.logger.Infof("Успешно снято %.2f со счета %s", amount, accountID)
	return nil
}
