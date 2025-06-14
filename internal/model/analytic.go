package model

import "time"

// AnalyticsRequest - запрос на получение аналитики
type AnalyticsRequest struct {
	StartDate    time.Time `json:"start_date"`
	EndDate      time.Time `json:"end_date"`
	ForecastDays int       `json:"forecast_days" validate:"lte=365"` // Максимум 365 дней
}

// FinancialStats - статистика по доходам/расходам
type FinancialStats struct {
	TotalIncome   float64                  `json:"total_income"`
	TotalExpenses float64                  `json:"total_expenses"`
	NetBalance    float64                  `json:"net_balance"`
	ByCategory    map[string]CategoryStats `json:"by_category"`
}

// CategoryStats - статистика по категориям
type CategoryStats struct {
	Income   float64 `json:"income"`
	Expenses float64 `json:"expenses"`
	Count    int     `json:"count"`
}

// CreditLoad - аналитика кредитной нагрузки
type CreditLoad struct {
	ActiveCredits     int     `json:"active_credits"`
	TotalDebt         float64 `json:"total_debt"`
	MonthlyPayments   float64 `json:"monthly_payments"`
	DebtToIncomeRatio float64 `json:"debt_to_income_ratio"`
}

// BalanceForecast - прогноз баланса
type BalanceForecast struct {
	Date             time.Time `json:"date"`
	ProjectedBalance float64   `json:"projected_balance"`
	PlannedPayments  float64   `json:"planned_payments"`
}
