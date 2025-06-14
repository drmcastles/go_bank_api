package handler

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	"banking-api/internal/model"
	"banking-api/internal/service"
)

type CreditHandler struct {
	creditService *service.CreditService
	logger        *logrus.Logger
}

func NewCreditHandler(creditService *service.CreditService, logger *logrus.Logger) *CreditHandler {
	return &CreditHandler{
		creditService: creditService,
		logger:        logger,
	}
}

func (h *CreditHandler) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("", h.CreateCredit).Methods("POST")
	router.HandleFunc("", h.GetUserCredits).Methods("GET")
	router.HandleFunc("/{creditId}/schedule", h.GetPaymentSchedule).Methods("GET")
	router.HandleFunc("/pay", h.MakePayment).Methods("POST") // Новый эндпоинт
}

func (h *CreditHandler) CreateCredit(w http.ResponseWriter, r *http.Request) {
	var req model.CreateCreditRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.WithError(err).Error("Failed to decode create credit request")
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	userID, ok := r.Context().Value("userID").(string)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	userUUID, err := uuid.Parse(userID)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	credit, err := h.creditService.CreateCredit(r.Context(), req, userUUID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to create credit")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(credit)
}

func (h *CreditHandler) GetUserCredits(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value("userID").(string)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	userUUID, err := uuid.Parse(userID)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	credits, err := h.creditService.GetUserCredits(r.Context(), userUUID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get user credits")
		http.Error(w, "Failed to get credits", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(credits)
}

func (h *CreditHandler) GetPaymentSchedule(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value("userID").(string)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	userUUID, err := uuid.Parse(userID)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	vars := mux.Vars(r)
	creditID, err := uuid.Parse(vars["creditId"])
	if err != nil {
		http.Error(w, "Invalid credit ID", http.StatusBadRequest)
		return
	}

	schedule, err := h.creditService.GetPaymentSchedule(r.Context(), creditID, userUUID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get payment schedule")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(schedule)
}

func (h *CreditHandler) MakePayment(w http.ResponseWriter, r *http.Request) {
	var req model.CreditPaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.WithError(err).Error("Ошибка декодирования запроса на платеж")
		http.Error(w, "Неверный формат запроса", http.StatusBadRequest)
		return
	}

	userID, ok := r.Context().Value("userID").(string)
	if !ok {
		http.Error(w, "Не авторизован", http.StatusUnauthorized)
		return
	}

	userUUID, err := uuid.Parse(userID)
	if err != nil {
		http.Error(w, "Неверный ID пользователя", http.StatusBadRequest)
		return
	}

	// Проверяем что кредит принадлежит пользователю
	credit, err := h.creditService.GetCreditByID(r.Context(), req.CreditID)
	if err != nil {
		h.logger.WithError(err).Error("Ошибка получения кредита")
		http.Error(w, "Кредит не найден", http.StatusNotFound)
		return
	}

	if credit.UserID != userUUID {
		http.Error(w, "Кредит не принадлежит пользователю", http.StatusForbidden)
		return
	}

	// Получаем следующий платеж по кредиту
	schedule, err := h.creditService.GetNextPayment(r.Context(), req.CreditID)
	if err != nil {
		h.logger.WithError(err).Error("Ошибка получения графика платежей")
		http.Error(w, "Ошибка получения платежа", http.StatusInternalServerError)
		return
	}

	// Выполняем платеж
	if err := h.creditService.ProcessPayment(r.Context(), schedule.ID, req.Amount); err != nil {
		h.logger.WithError(err).Error("Ошибка выполнения платежа")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "Платеж выполнен"})
}
