package service

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"banking-api/internal/model"
	"banking-api/internal/repository"
)

type CreditService struct {
	userRepo        *repository.UserRepository
	creditRepo      *repository.CreditRepository
	accountRepo     *repository.AccountRepository
	transactionRepo *repository.TransactionRepository
	emailSender     *EmailSender
	cbrClient       *CBRClient
	logger          *logrus.Logger
}

func NewCreditService(
	userRepo *repository.UserRepository,
	creditRepo *repository.CreditRepository,
	accountRepo *repository.AccountRepository,
	transactionRepo *repository.TransactionRepository,
	emailSender *EmailSender,
	cbrClient *CBRClient,
	logger *logrus.Logger,
) *CreditService {
	return &CreditService{
		userRepo:        userRepo,
		creditRepo:      creditRepo,
		accountRepo:     accountRepo,
		transactionRepo: transactionRepo,
		emailSender:     emailSender,
		cbrClient:       cbrClient,
		logger:          logger,
	}
}

// CalculateMonthlyPayment рассчитывает аннуитетный платеж
func (s *CreditService) CalculateMonthlyPayment(amount float64, termMonths int, interestRate float64) float64 {
	monthlyRate := interestRate / 12 / 100
	annuityCoeff := (monthlyRate * math.Pow(1+monthlyRate, float64(termMonths))) /
		(math.Pow(1+monthlyRate, float64(termMonths)) - 1)
	return amount * annuityCoeff
}

func (s *CreditService) CreateCredit(ctx context.Context, req model.CreateCreditRequest, userID uuid.UUID) (*model.Credit, error) {
	s.logger.Infof("Создание кредита для пользователя %s, сумма: %.2f, срок: %d мес.",
		userID, req.Amount, req.TermMonths)

	// Получаем счет и проверяем владельца
	account, err := s.accountRepo.GetByID(ctx, req.AccountID)
	if err != nil {
		s.logger.WithError(err).Errorf("Ошибка получения счета %s", req.AccountID)
		return nil, fmt.Errorf("ошибка получения счета: %w", err)
	}

	if account.UserID != userID {
		s.logger.Warnf("Попытка создания кредита на чужой счет: пользователь %s, владелец счета %s",
			userID, account.UserID)
		return nil, fmt.Errorf("счет не принадлежит пользователю")
	}

	// Получаем текущую ставку ЦБ
	rate, err := s.cbrClient.GetCentralBankRate()
	if err != nil {
		s.logger.WithError(err).Warn("Не удалось получить ставку ЦБ, используется значение по умолчанию")
		rate = 22.0 // дефолтная ставка, если ЦБ недоступен
	}

	// Добавляем маржу к ключевой ставке
	interestRate := rate + 5.0 // маржа 5%
	s.logger.Infof("Рассчитанная ставка по кредиту: %.2f%% (ставка ЦБ: %.2f%%, маржа: 5%%)",
		interestRate, rate)

	// Рассчитываем ежемесячный платеж
	monthlyPayment := s.CalculateMonthlyPayment(req.Amount, req.TermMonths, interestRate)
	s.logger.Infof("Ежемесячный платеж: %.2f, сумма кредита: %.2f, срок: %d мес.",
		monthlyPayment, req.Amount, req.TermMonths)

	now := time.Now()
	endDate := now.AddDate(0, req.TermMonths, 0)

	credit := &model.Credit{
		ID:             uuid.New(),
		AccountID:      req.AccountID,
		UserID:         userID,
		Amount:         req.Amount,
		InterestRate:   interestRate,
		TermMonths:     req.TermMonths,
		MonthlyPayment: monthlyPayment,
		StartDate:      now,
		EndDate:        endDate,
		Status:         "active",
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	// Начинаем транзакцию
	db := s.creditRepo.GetDB()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		s.logger.WithError(err).Error("Ошибка начала транзакции")
		return nil, fmt.Errorf("ошибка начала транзакции: %w", err)
	}
	defer tx.Rollback()

	// Зачисляем сумму кредита на счет
	if err := s.accountRepo.UpdateBalanceTx(ctx, tx, req.AccountID, req.Amount); err != nil {
		s.logger.WithError(err).Errorf("Ошибка зачисления средств на счет %s", req.AccountID)
		return nil, fmt.Errorf("ошибка зачисления средств: %w", err)
	}

	// Создаем запись о кредите
	if err := s.creditRepo.CreateCredit(ctx, credit); err != nil {
		s.logger.WithError(err).Error("Ошибка создания записи о кредите")
		return nil, fmt.Errorf("ошибка создания кредита: %w", err)
	}

	// Генерируем график платежей
	if err := s.generatePaymentSchedule(ctx, credit); err != nil {
		s.logger.WithError(err).Error("Ошибка генерации графика платежей")
		return nil, fmt.Errorf("ошибка создания графика платежей: %w", err)
	}

	// Создаем запись о транзакции
	transactionID := uuid.New()
	transaction := &model.Transaction{
		ID:              transactionID,
		AccountID:       req.AccountID,
		Amount:          req.Amount,
		TransactionType: model.TransactionTypeCredit,
		ReferenceID:     &credit.ID,
		CreatedAt:       now,
	}

	if err := s.transactionRepo.CreateTx(ctx, tx, transaction); err != nil {
		s.logger.WithError(err).Error("Ошибка создания транзакции")
		return nil, fmt.Errorf("ошибка записи транзакции: %w", err)
	}

	// Коммитим транзакцию
	if err := tx.Commit(); err != nil {
		s.logger.WithError(err).Error("Ошибка подтверждения транзакции")
		return nil, fmt.Errorf("ошибка подтверждения операции: %w", err)
	}

	s.logger.Infof("Кредит %s успешно создан для пользователя %s", credit.ID, userID)
	return credit, nil
}

func (s *CreditService) generatePaymentSchedule(ctx context.Context, credit *model.Credit) error {
	s.logger.Infof("Генерация графика платежей для кредита %s", credit.ID)
	remainingPrincipal := credit.Amount
	monthlyRate := credit.InterestRate / 12 / 100

	for i := 1; i <= credit.TermMonths; i++ {
		interest := remainingPrincipal * monthlyRate
		principal := credit.MonthlyPayment - interest
		if i == credit.TermMonths {
			// Корректировка последнего платежа для устранения погрешностей округления
			principal = remainingPrincipal
		}

		paymentDate := credit.StartDate.AddDate(0, i, 0)
		now := time.Now()

		schedule := &model.PaymentSchedule{
			ID:            uuid.New(),
			CreditID:      credit.ID,
			PaymentNumber: i,
			PaymentDate:   paymentDate,
			Amount:        credit.MonthlyPayment,
			Principal:     principal,
			Interest:      interest,
			Status:        "pending",
			CreatedAt:     now,
			UpdatedAt:     now,
		}

		if err := s.creditRepo.CreatePaymentSchedule(ctx, schedule); err != nil {
			s.logger.WithError(err).Errorf("Ошибка создания записи о платеже №%d", i)
			return fmt.Errorf("ошибка создания платежа: %w", err)
		}

		remainingPrincipal -= principal
	}

	s.logger.Infof("График платежей для кредита %s успешно сгенерирован (%d платежей)",
		credit.ID, credit.TermMonths)
	return nil
}

func (s *CreditService) GetUserCredits(ctx context.Context, userID uuid.UUID) ([]model.Credit, error) {
	s.logger.Infof("Получение списка кредитов пользователя %s", userID)
	credits, err := s.creditRepo.GetUserCredits(ctx, userID)
	if err != nil {
		s.logger.WithError(err).Error("Ошибка получения кредитов пользователя")
		return nil, fmt.Errorf("ошибка получения кредитов: %w", err)
	}
	return credits, nil
}

func (s *CreditService) GetPaymentSchedule(ctx context.Context, creditID uuid.UUID, userID uuid.UUID) ([]model.PaymentSchedule, error) {
	s.logger.Infof("Получение графика платежей для кредита %s (пользователь %s)", creditID, userID)

	// Проверяем принадлежность кредита пользователю
	credit, err := s.creditRepo.GetCreditByID(ctx, creditID)
	if err != nil {
		s.logger.WithError(err).Errorf("Ошибка получения кредита %s", creditID)
		return nil, fmt.Errorf("ошибка получения кредита: %w", err)
	}

	if credit.UserID != userID {
		s.logger.Warnf("Попытка получения графика платежей чужого кредита: пользователь %s, владелец %s",
			userID, credit.UserID)
		return nil, fmt.Errorf("кредит не принадлежит пользователю")
	}

	schedule, err := s.creditRepo.GetPaymentSchedule(ctx, creditID)
	if err != nil {
		s.logger.WithError(err).Errorf("Ошибка получения графика платежей для кредита %s", creditID)
		return nil, fmt.Errorf("ошибка получения графика платежей: %w", err)
	}

	return schedule, nil
}

func (s *CreditService) ProcessPayments(ctx context.Context) error {
	s.logger.Info("Автоматическая обработка платежей по кредитам")
	pendingPayments, err := s.creditRepo.GetPendingPayments(ctx, time.Now())
	if err != nil {
		s.logger.WithError(err).Error("Ошибка получения ожидающих платежей")
		return fmt.Errorf("ошибка получения платежей: %w", err)
	}

	s.logger.Infof("Найдено %d платежей для обработки", len(pendingPayments))
	for _, payment := range pendingPayments {
		if err := s.processPayment(ctx, payment); err != nil {
			s.logger.WithError(err).Errorf("Ошибка обработки платежа %s", payment.ID)
			continue
		}
	}

	return nil
}

func (s *CreditService) processPayment(ctx context.Context, payment model.PaymentSchedule) error {
	s.logger.Infof("Обработка платежа %s по кредиту %s", payment.ID, payment.CreditID)

	credit, err := s.creditRepo.GetCreditByID(ctx, payment.CreditID)
	if err != nil {
		return fmt.Errorf("ошибка получения кредита: %w", err)
	}

	// Начинаем транзакцию ДО получения счета
	db := s.creditRepo.GetDB()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("ошибка начала транзакции: %w", err)
	}
	defer tx.Rollback()

	// Получаем счет ВНУТРИ транзакции с блокировкой
	account, err := s.accountRepo.GetByIDForUpdate(ctx, tx, credit.AccountID)
	if err != nil {
		return fmt.Errorf("ошибка получения счета: %w", err)
	}

	var status string
	var paidAt *time.Time
	var penalty float64

	if account.Balance >= payment.Amount {
		if err := s.accountRepo.UpdateBalanceTx(ctx, tx, account.ID, -payment.Amount); err != nil {
			return fmt.Errorf("ошибка списания средств: %w", err)
		}
		status = "paid"
		now := time.Now()
		paidAt = &now
	} else {
		penalty = payment.Amount * 0.1
		status = "overdue"
	}

	// Обновляем статус платежа
	if err := s.creditRepo.UpdatePaymentStatus(ctx, payment.ID, status, paidAt); err != nil {
		s.logger.WithError(err).Errorf("Ошибка обновления статуса платежа %s", payment.ID)
		return fmt.Errorf("ошибка обновления платежа: %w", err)
	}

	// Если платеж успешен, проверяем полностью ли погашен кредит
	if status == "paid" {
		remainingPayments, err := s.creditRepo.GetPaymentSchedule(ctx, credit.ID)
		if err != nil {
			s.logger.WithError(err).Errorf("Ошибка получения оставшихся платежей по кредиту %s", credit.ID)
			return fmt.Errorf("ошибка получения платежей: %w", err)
		}

		allPaid := true
		for _, p := range remainingPayments {
			if p.Status != "paid" && p.ID != payment.ID {
				allPaid = false
				break
			}
		}

		if allPaid {
			if err := s.creditRepo.UpdateCreditStatus(ctx, credit.ID, "paid"); err != nil {
				s.logger.WithError(err).Errorf("Ошибка обновления статуса кредита %s", credit.ID)
				return fmt.Errorf("ошибка обновления кредита: %w", err)
			}
			s.logger.Infof("Кредит %s полностью погашен", credit.ID)
		}
	}

	// Создаем запись о транзакции
	transactionID := uuid.New()
	now := time.Now()

	transaction := &model.Transaction{
		ID:              transactionID,
		AccountID:       account.ID,
		Amount:          payment.Amount + penalty,
		TransactionType: model.TransactionTypeCreditPayment,
		ReferenceID:     &payment.ID,
		CreatedAt:       now,
	}

	if err := s.transactionRepo.CreateTx(ctx, tx, transaction); err != nil {
		s.logger.WithError(err).Error("Ошибка создания записи о транзакции")
		return fmt.Errorf("ошибка записи транзакции: %w", err)
	}

	// Коммитим транзакцию
	if err := tx.Commit(); err != nil {
		s.logger.WithError(err).Error("Ошибка подтверждения транзакции")
		return fmt.Errorf("ошибка подтверждения операции: %w", err)
	}

	// Отправка email уведомления
	if status == "paid" {
		// Получаем email пользователя
		user, err := s.userRepo.GetByID(ctx, credit.UserID)
		if err == nil && user.Email != "" {
			go func() {
				if err := s.emailSender.SendCreditPaymentNotification(
					user.Email,
					payment.Amount,
					credit.ID,
				); err != nil {
					s.logger.WithError(err).Warn("Не удалось отправить email уведомление")
				}
			}()
		}
	}

	return nil
}

func (s *CreditService) GetNextPayment(ctx context.Context, creditID uuid.UUID) (*model.PaymentSchedule, error) {
	s.logger.Infof("Получение следующего платежа по кредиту %s", creditID)
	payments, err := s.creditRepo.GetPaymentSchedule(ctx, creditID)
	if err != nil {
		s.logger.WithError(err).Errorf("Ошибка получения графика платежей для кредита %s", creditID)
		return nil, fmt.Errorf("ошибка получения платежей: %w", err)
	}

	for _, p := range payments {
		if p.Status == "pending" {
			s.logger.Infof("Найден ожидающий платеж %s по кредиту %s", p.ID, creditID)
			return &p, nil
		}
	}

	s.logger.Infof("Нет ожидающих платежей по кредиту %s", creditID)
	return nil, fmt.Errorf("нет ожидающих платежей")
}

func (s *CreditService) ProcessPayment(ctx context.Context, paymentID uuid.UUID, amount float64) error {
	s.logger.Infof("Ручная обработка платежа %s на сумму %.2f", paymentID, amount)
	payment, err := s.creditRepo.GetPaymentByID(ctx, paymentID)
	if err != nil {
		s.logger.WithError(err).Errorf("Ошибка получения платежа %s", paymentID)
		return fmt.Errorf("ошибка получения платежа: %w", err)
	}

	if amount < payment.Amount {
		s.logger.Warnf("Недостаточная сумма платежа: внесено %.2f, требуется %.2f", amount, payment.Amount)
		return fmt.Errorf("сумма платежа меньше требуемой")
	}

	// Используем логику из шедулера
	return s.processPayment(ctx, *payment)
}

// GetCreditByID возвращает кредит по ID с проверкой принадлежности пользователю
func (s *CreditService) GetCreditByID(ctx context.Context, creditID uuid.UUID) (*model.Credit, error) {
	s.logger.Infof("Получение кредита %s", creditID)
	credit, err := s.creditRepo.GetCreditByID(ctx, creditID)
	if err != nil {
		s.logger.WithError(err).Errorf("Ошибка получения кредита %s", creditID)
		return nil, fmt.Errorf("ошибка получения кредита: %w", err)
	}
	return credit, nil
}
