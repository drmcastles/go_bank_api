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

// AccountHandler обрабатывает запросы, связанные с аккаунтами
type AccountHandler struct {
	accountService *service.AccountService // Сервис для работы с аккаунтами
	logger         *logrus.Logger          // Логгер
}

// NewAccountHandler создает новый AccountHandler
func NewAccountHandler(accountService *service.AccountService, logger *logrus.Logger) *AccountHandler {
	return &AccountHandler{
		accountService: accountService,
		logger:         logger,
	}
}

// RegisterRoutes регистрирует маршруты для работы с аккаунтами
func (h *AccountHandler) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("", h.CreateAccount).Methods("POST")     // Маршрут для создания аккаунта
	router.HandleFunc("", h.GetUserAccounts).Methods("GET")    // Маршрут для получения аккаунтов пользователя
	router.HandleFunc("/transfer", h.Transfer).Methods("POST") // Маршрут для перевода средств
	router.HandleFunc("/deposit", h.Deposit).Methods("POST")   // Маршрут для пополнения счета
	router.HandleFunc("/credit", h.Credit).Methods("POST")     // Маршрут для снятия средств
}

// CreateAccount обрабатывает запрос на создание нового аккаунта
func (h *AccountHandler) CreateAccount(w http.ResponseWriter, r *http.Request) {
	var req model.CreateAccountRequest
	// Декодируем входные данные
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.WithError(err).Error("Не удалось декодировать запрос на создание аккаунта")
		http.Error(w, "Неверный формат запроса", http.StatusBadRequest)
		return
	}

	// Получаем userID из контекста
	userID, ok := r.Context().Value("userID").(string)
	if !ok {
		http.Error(w, "Неавторизованный доступ", http.StatusUnauthorized)
		return
	}

	// Парсим userID в UUID
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		http.Error(w, "Неверный идентификатор пользователя", http.StatusBadRequest)
		return
	}

	// Создаем аккаунт
	account, err := h.accountService.CreateAccount(r.Context(), userUUID, req.Currency)
	if err != nil {
		h.logger.WithError(err).Error("Не удалось создать аккаунт")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Формируем ответ
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(account) // Отправляем ответ
}

// GetUserAccounts обрабатывает запрос на получение аккаунтов пользователя
func (h *AccountHandler) GetUserAccounts(w http.ResponseWriter, r *http.Request) {
	// Получаем userID из контекста
	userID, ok := r.Context().Value("userID").(string)
	if !ok {
		http.Error(w, "Неавторизованный доступ", http.StatusUnauthorized)
		return
	}

	// Парсим userID в UUID
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		http.Error(w, "Неверный идентификатор пользователя", http.StatusBadRequest)
		return
	}

	// Получаем аккаунты пользователя
	accounts, err := h.accountService.GetUserAccounts(r.Context(), userUUID)
	if err != nil {
		h.logger.WithError(err).Error("Не удалось получить аккаунты пользователя")
		http.Error(w, "Не удалось получить аккаунты", http.StatusInternalServerError)
		return
	}

	// Формируем ответ
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(accounts) // Отправляем ответ
}

// Transfer обрабатывает запрос на перевод средств
func (h *AccountHandler) Transfer(w http.ResponseWriter, r *http.Request) {
	var req model.TransferRequest
	// Декодируем входные данные
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.WithError(err).Error("Не удалось декодировать запрос на перевод")
		http.Error(w, "Неверный формат запроса", http.StatusBadRequest)
		return
	}

	// Получаем userID из контекста
	userID, ok := r.Context().Value("userID").(string)
	if !ok {
		http.Error(w, "Неавторизованный доступ", http.StatusUnauthorized)
		return
	}

	// Парсим userID в UUID
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		http.Error(w, "Неверный идентификатор пользователя", http.StatusBadRequest)
		return
	}

	// Выполняем перевод средств
	if err := h.accountService.Transfer(r.Context(), req.FromAccountID, req.ToAccountID, req.Amount, userUUID); err != nil {
		h.logger.WithError(err).Error("Не удалось выполнить перевод средств")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Успешный ответ
	w.WriteHeader(http.StatusOK)
}

// Deposit обрабатывает запрос на пополнение счета
func (h *AccountHandler) Deposit(w http.ResponseWriter, r *http.Request) {
	var req model.ChangeRequest
	// Декодируем входные данные
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.WithError(err).Error("Не удалось декодировать запрос на пополнение")
		http.Error(w, "Неверный формат запроса", http.StatusBadRequest)
		return
	}

	// Получаем userID из контекста
	userID, ok := r.Context().Value("userID").(string)
	if !ok {
		http.Error(w, "Неавторизованный доступ", http.StatusUnauthorized)
		return
	}

	// Парсим userID в UUID
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		http.Error(w, "Неверный идентификатор пользователя", http.StatusBadRequest)
		return
	}

	// Выполняем пополнение счета
	if err := h.accountService.Deposit(r.Context(), req.AccountID, req.Amount, userUUID); err != nil {
		h.logger.WithError(err).Error("Не удалось пополнить счет")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Успешный ответ
	w.WriteHeader(http.StatusOK)
}

// Credit обрабатывает запрос на снятие средств
func (h *AccountHandler) Credit(w http.ResponseWriter, r *http.Request) {
	var req model.ChangeRequest
	// Декодируем входные данные
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.WithError(err).Error("Не удалось декодировать запрос на снятие")
		http.Error(w, "Неверный формат запроса", http.StatusBadRequest)
		return
	}

	// Получаем userID из контекста
	userID, ok := r.Context().Value("userID").(string)
	if !ok {
		http.Error(w, "Неавторизованный доступ", http.StatusUnauthorized)
		return
	}

	// Парсим userID в UUID
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		http.Error(w, "Неверный идентификатор пользователя", http.StatusBadRequest)
		return
	}

	// Выполняем снятие средств
	if err := h.accountService.Withdraw(r.Context(), req.AccountID, req.Amount, userUUID); err != nil {
		h.logger.WithError(err).Error("Не удалось снять средства")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Успешный ответ
	w.WriteHeader(http.StatusOK)
}
