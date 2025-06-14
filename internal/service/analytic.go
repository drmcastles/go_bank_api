package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"banking-api/internal/model"
	"banking-api/internal/repository"
)

type AnalyticService struct {
	transactionRepo *repository.TransactionRepository
	creditRepo      *repository.CreditRepository
	accountRepo     *repository.AccountRepository
	logger          *logrus.Logger
}

func NewAnalyticService(
	transactionRepo *repository.TransactionRepository,
	creditRepo *repository.CreditRepository,
	accountRepo *repository.AccountRepository,
	logger *logrus.Logger,
) *AnalyticService {
	return &AnalyticService{
		transactionRepo: transactionRepo,
		creditRepo:      creditRepo,
		accountRepo:     accountRepo,
		logger:          logger,
	}
}

// GetFinancialStats возвращает статистику по доходам/расходам за период
func (s *AnalyticService) GetFinancialStats(
	ctx context.Context,
	userID uuid.UUID,
	startDate, endDate time.Time,
) (*model.FinancialStats, error) {
	// Добавляем логирование входящих параметров
	s.logger.WithFields(logrus.Fields{
		"user_id":    userID,
		"start_date": startDate.Format("2006-01-02"),
		"end_date":   endDate.Format("2006-01-02"),
	}).Debug("Начало расчета финансовой статистики")

	// Валидация дат
	if startDate.After(endDate) {
		s.logger.Warn("Дата начала периода позже даты окончания")
		return nil, fmt.Errorf("дата начала не может быть позже даты окончания")
	}

	// Получаем все счета пользователя
	accounts, err := s.accountRepo.GetUserAccounts(ctx, userID)
	if err != nil {
		s.logger.WithError(err).Error("Ошибка получения счетов пользователя")
		return nil, fmt.Errorf("не удалось получить счета пользователя: %w", err)
	}

	if len(accounts) == 0 {
		s.logger.Info("У пользователя нет счетов для анализа")
		return &model.FinancialStats{
			ByCategory: make(map[string]model.CategoryStats),
		}, nil
	}

	// Получаем транзакции по всем счетам за период
	var allTransactions []model.Transaction
	for _, acc := range accounts {
		transactions, err := s.transactionRepo.GetByAccountAndPeriod(ctx, acc.ID, startDate, endDate)
		if err != nil {
			s.logger.WithFields(logrus.Fields{
				"error":      err.Error(),
				"account_id": acc.ID,
			}).Error("Ошибка получения транзакций по счету")
			continue
		}
		allTransactions = append(allTransactions, transactions...)
	}

	s.logger.WithField("transaction_count", len(allTransactions)).Debug("Получены транзакции для анализа")

	// Анализируем транзакции
	stats := &model.FinancialStats{
		ByCategory: make(map[string]model.CategoryStats),
	}

	for _, tx := range allTransactions {
		category := string(tx.TransactionType)
		if _, exists := stats.ByCategory[category]; !exists {
			stats.ByCategory[category] = model.CategoryStats{}
		}

		categoryStats := stats.ByCategory[category]

		if tx.Amount > 0 {
			stats.TotalIncome += tx.Amount
			categoryStats.Income += tx.Amount
		} else {
			amount := -tx.Amount // Преобразуем отрицательную сумму в положительную
			stats.TotalExpenses += amount
			categoryStats.Expenses += amount
		}
		categoryStats.Count++
		stats.ByCategory[category] = categoryStats
	}

	stats.NetBalance = stats.TotalIncome - stats.TotalExpenses

	// Детальное логирование результатов
	s.logger.WithFields(logrus.Fields{
		"income":       stats.TotalIncome,
		"expenses":     stats.TotalExpenses,
		"balance":      stats.NetBalance,
		"categories":   len(stats.ByCategory),
		"transactions": len(allTransactions),
	}).Info("Финансовая статистика успешно рассчитана")

	return stats, nil
}

// GetCreditLoad возвращает аналитику кредитной нагрузки
func (s *AnalyticService) GetCreditLoad(
	ctx context.Context,
	userID uuid.UUID,
) (*model.CreditLoad, error) {
	s.logger.WithField("user_id", userID).Info("Расчет кредитной нагрузки")

	// Получаем активные кредиты пользователя
	credits, err := s.creditRepo.GetUserCredits(ctx, userID)
	if err != nil {
		s.logger.WithError(err).Error("Ошибка получения кредитов пользователя")
		return nil, fmt.Errorf("ошибка получения кредитов: %w", err)
	}

	load := &model.CreditLoad{}
	var activeCredits []model.Credit

	// Фильтруем активные кредиты
	for _, credit := range credits {
		if credit.Status == "active" {
			activeCredits = append(activeCredits, credit)
			load.TotalDebt += credit.Amount
			load.MonthlyPayments += credit.MonthlyPayment
		}
	}

	load.ActiveCredits = len(activeCredits)

	// Рассчитываем отношение долга к доходу (D/I ratio)
	if load.MonthlyPayments > 0 {
		// Получаем среднемесячный доход за последние 3 месяца
		endDate := time.Now()
		startDate := endDate.AddDate(0, -3, 0)
		stats, err := s.GetFinancialStats(ctx, userID, startDate, endDate)
		if err != nil {
			s.logger.WithError(err).Warn("Не удалось рассчитать доход для D/I ratio")
		} else if stats.TotalIncome > 0 {
			avgMonthlyIncome := stats.TotalIncome / 3
			load.DebtToIncomeRatio = load.MonthlyPayments / avgMonthlyIncome
		}
	}

	s.logger.WithFields(logrus.Fields{
		"active_credits":   load.ActiveCredits,
		"total_debt":       load.TotalDebt,
		"monthly_payments": load.MonthlyPayments,
		"debt_to_income":   load.DebtToIncomeRatio,
	}).Info("Кредитная нагрузка рассчитана")

	return load, nil
}

// GetBalanceForecast возвращает прогноз баланса на указанное количество дней
func (s *AnalyticService) GetBalanceForecast(
	ctx context.Context,
	userID uuid.UUID,
	days int,
) ([]model.BalanceForecast, error) {
	s.logger.WithFields(logrus.Fields{
		"user_id": userID,
		"days":    days,
	}).Info("Расчет прогноза баланса")

	if days <= 0 || days > 365 {
		return nil, fmt.Errorf("период прогноза должен быть от 1 до 365 дней")
	}

	// Получаем текущие балансы счетов
	accounts, err := s.accountRepo.GetUserAccounts(ctx, userID)
	if err != nil {
		s.logger.WithError(err).Error("Ошибка получения счетов пользователя")
		return nil, fmt.Errorf("ошибка получения счетов: %w", err)
	}

	// Рассчитываем общий текущий баланс
	var currentBalance float64
	for _, acc := range accounts {
		currentBalance += acc.Balance
	}

	// Получаем запланированные платежи (кредиты и другие)
	now := time.Now()
	endDate := now.AddDate(0, 0, days)
	plannedPayments, err := s.getPlannedPayments(ctx, userID, now, endDate)
	if err != nil {
		s.logger.WithError(err).Error("Ошибка получения запланированных платежей")
		return nil, fmt.Errorf("ошибка получения платежей: %w", err)
	}

	// Строим прогноз по дням
	forecast := make([]model.BalanceForecast, 0, days)
	runningBalance := currentBalance

	for day := 0; day < days; day++ {
		date := now.AddDate(0, 0, day)
		dailyPayments := 0.0

		// Суммируем платежи на эту дату
		if payments, ok := plannedPayments[date]; ok {
			for _, amount := range payments {
				dailyPayments += amount
			}
		}

		runningBalance -= dailyPayments
		forecast = append(forecast, model.BalanceForecast{
			Date:             date,
			ProjectedBalance: runningBalance,
			PlannedPayments:  dailyPayments,
		})
	}

	s.logger.WithFields(logrus.Fields{
		"start_balance": currentBalance,
		"end_balance":   runningBalance,
		"days":          days,
	}).Info("Прогноз баланса рассчитан")

	return forecast, nil
}

// getPlannedPayments возвращает запланированные платежи по датам
func (s *AnalyticService) getPlannedPayments(
	ctx context.Context,
	userID uuid.UUID,
	startDate, endDate time.Time,
) (map[time.Time][]float64, error) {
	payments := make(map[time.Time][]float64)

	// Получаем платежи по кредитам
	credits, err := s.creditRepo.GetUserCredits(ctx, userID)
	if err != nil {
		return nil, err
	}

	for _, credit := range credits {
		if credit.Status != "active" {
			continue
		}

		schedule, err := s.creditRepo.GetPaymentSchedule(ctx, credit.ID)
		if err != nil {
			s.logger.WithError(err).Errorf("Ошибка получения графика платежей для кредита %s", credit.ID)
			continue
		}

		for _, payment := range schedule {
			if payment.Status == "pending" &&
				!payment.PaymentDate.Before(startDate) &&
				!payment.PaymentDate.After(endDate) {
				payments[payment.PaymentDate] = append(payments[payment.PaymentDate], payment.Amount)
			}
		}
	}

	return payments, nil
}
