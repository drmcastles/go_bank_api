package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	"banking-api/internal/service"
)

type AnalyticsHandler struct {
	accountService  *service.AccountService
	creditService   *service.CreditService
	analyticService *service.AnalyticService
	logger          *logrus.Logger
}

func NewAnalyticsHandler(
	accountService *service.AccountService,
	creditService *service.CreditService,
	analyticService *service.AnalyticService,
	logger *logrus.Logger,
) *AnalyticsHandler {
	return &AnalyticsHandler{
		accountService:  accountService,
		creditService:   creditService,
		analyticService: analyticService,
		logger:          logger,
	}
}

func (h *AnalyticsHandler) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/stats", h.GetFinancialStats).Methods("GET")
	router.HandleFunc("/credit-load", h.GetCreditLoad).Methods("GET")
	router.HandleFunc("/forecast", h.GetBalanceForecast).Methods("GET")
}

// GetFinancialStats возвращает статистику по доходам/расходам
func (h *AnalyticsHandler) GetFinancialStats(w http.ResponseWriter, r *http.Request) {
	// Получаем userID из контекста
	userID, ok := r.Context().Value("userID").(string)
	if !ok {
		h.logger.Warn("Попытка получения аналитики без авторизации")
		http.Error(w, "Требуется авторизация", http.StatusUnauthorized)
		return
	}

	userUUID, err := uuid.Parse(userID)
	if err != nil {
		h.logger.WithField("userID", userID).Warn("Неверный формат ID пользователя")
		http.Error(w, "Неверный ID пользователя", http.StatusBadRequest)
		return
	}

	// Парсим параметры запроса
	startDate, endDate, err := h.parseDateRange(r)
	if err != nil {
		h.logger.WithError(err).Warn("Неверные параметры даты")
		http.Error(w, "Неверный формат даты (используйте YYYY-MM-DD)", http.StatusBadRequest)
		return
	}

	h.logger.WithFields(logrus.Fields{
		"user_id":    userUUID,
		"start_date": startDate,
		"end_date":   endDate,
	}).Info("Запрос финансовой статистики")

	// Получаем статистику
	stats, err := h.analyticService.GetFinancialStats(r.Context(), userUUID, startDate, endDate)
	if err != nil {
		h.logger.WithError(err).Error("Ошибка получения финансовой статистики")
		http.Error(w, "Ошибка получения статистики", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(stats); err != nil {
		h.logger.WithError(err).Error("Ошибка кодирования финансовой статистики")
	}
}

// GetCreditLoad возвращает аналитику кредитной нагрузки
func (h *AnalyticsHandler) GetCreditLoad(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value("userID").(string)
	if !ok {
		h.logger.Warn("Попытка получения кредитной нагрузки без авторизации")
		http.Error(w, "Требуется авторизация", http.StatusUnauthorized)
		return
	}

	userUUID, err := uuid.Parse(userID)
	if err != nil {
		h.logger.WithField("userID", userID).Warn("Неверный формат ID пользователя")
		http.Error(w, "Неверный ID пользователя", http.StatusBadRequest)
		return
	}

	h.logger.WithField("user_id", userUUID).Info("Запрос кредитной нагрузки")

	load, err := h.analyticService.GetCreditLoad(r.Context(), userUUID)
	if err != nil {
		h.logger.WithError(err).Error("Ошибка получения кредитной нагрузки")
		http.Error(w, "Ошибка получения кредитной нагрузки", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(load); err != nil {
		h.logger.WithError(err).Error("Ошибка кодирования кредитной нагрузки")
	}
}

// GetBalanceForecast возвращает прогноз баланса
func (h *AnalyticsHandler) GetBalanceForecast(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value("userID").(string)
	if !ok {
		h.logger.Warn("Попытка получения прогноза баланса без авторизации")
		http.Error(w, "Требуется авторизация", http.StatusUnauthorized)
		return
	}

	userUUID, err := uuid.Parse(userID)
	if err != nil {
		h.logger.WithField("userID", userID).Warn("Неверный формат ID пользователя")
		http.Error(w, "Неверный ID пользователя", http.StatusBadRequest)
		return
	}

	// Парсим параметры запроса
	days := 30 // значение по умолчанию
	if daysParam := r.URL.Query().Get("days"); daysParam != "" {
		if d, err := strconv.Atoi(daysParam); err == nil && d > 0 && d <= 365 {
			days = d
		}
	}

	h.logger.WithFields(logrus.Fields{
		"user_id": userUUID,
		"days":    days,
	}).Info("Запрос прогноза баланса")

	// Получаем прогноз
	forecast, err := h.analyticService.GetBalanceForecast(r.Context(), userUUID, days)
	if err != nil {
		h.logger.WithError(err).Error("Ошибка получения прогноза баланса")
		http.Error(w, "Ошибка получения прогноза", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(forecast); err != nil {
		h.logger.WithError(err).Error("Ошибка кодирования прогноза баланса")
	}
}

// parseDateRange парсит даты из параметров запроса
func (h *AnalyticsHandler) parseDateRange(r *http.Request) (time.Time, time.Time, error) {
	now := time.Now()
	startDate := now.AddDate(0, -1, 0) // по умолчанию последний месяц
	endDate := now

	if startParam := r.URL.Query().Get("start"); startParam != "" {
		if t, err := time.Parse("2006-01-02", startParam); err == nil {
			startDate = t
		} else {
			return time.Time{}, time.Time{}, err
		}
	}

	if endParam := r.URL.Query().Get("end"); endParam != "" {
		if t, err := time.Parse("2006-01-02", endParam); err == nil {
			endDate = t
		} else {
			return time.Time{}, time.Time{}, err
		}
	}

	// Проверяем что startDate <= endDate
	if startDate.After(endDate) {
		return endDate, startDate, nil
	}

	return startDate, endDate, nil
}
