package handler

import (
	"encoding/json"
	"github.com/sirupsen/logrus"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/gorilla/mux"

	"banking-api/internal/model"
	"banking-api/internal/service"
)

type CardHandler struct {
	cardService *service.CardService
	logger      *logrus.Logger
}

func NewCardHandler(cardService *service.CardService, logger *logrus.Logger) *CardHandler {
	return &CardHandler{
		cardService: cardService,
		logger:      logger,
	}
}

func (h *CardHandler) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("", h.CreateCard).Methods("POST")
	router.HandleFunc("", h.ListCards).Methods("GET")
	router.HandleFunc("/{id}", h.GetCard).Methods("GET")
	router.HandleFunc("/payments", h.ProcessPayment).Methods("POST")
}

func (h *CardHandler) CreateCard(w http.ResponseWriter, r *http.Request) {
	// Получаем ID пользователя из контекста
	userID, ok := r.Context().Value("userID").(string)
	if !ok {
		h.logger.Warn("Попытка создания карты без авторизации")
		http.Error(w, "Требуется авторизация", http.StatusUnauthorized)
		return
	}

	userUUID, err := uuid.Parse(userID)
	if err != nil {
		h.logger.WithField("userID", userID).Warn("Неверный формат ID пользователя")
		http.Error(w, "Неверный ID пользователя", http.StatusBadRequest)
		return
	}

	// Декодируем тело запроса
	var req model.CardRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.WithError(err).Warn("Ошибка декодирования запроса")
		http.Error(w, "Неверный формат запроса", http.StatusBadRequest)
		return
	}

	// Проверяем обязательные поля
	if strings.TrimSpace(req.Name) == "" {
		h.logger.Warn("Попытка создания карты без указания имени")
		http.Error(w, "Имя карты обязательно", http.StatusBadRequest)
		return
	}

	h.logger.WithFields(logrus.Fields{
		"userID":    userUUID,
		"accountID": req.AccountID,
	}).Info("Попытка создания новой карты")

	// Создаем карту через сервис
	card, err := h.cardService.CreateCard(r.Context(), userUUID, &req)
	if err != nil {
		h.logger.WithFields(logrus.Fields{
			"error":      err.Error(),
			"user_id":    userUUID,
			"account_id": req.AccountID,
		}).Error("Ошибка создания карты")

		switch {
		case strings.Contains(err.Error(), "account verification"):
			http.Error(w, "Неверный счет", http.StatusBadRequest)
		case strings.Contains(err.Error(), "encryption"):
			http.Error(w, "Ошибка шифрования данных карты", http.StatusInternalServerError)
		default:
			http.Error(w, "Ошибка создания карты", http.StatusInternalServerError)
		}
		return
	}

	h.logger.WithField("cardID", card.ID).Info("Карта успешно создана")

	// Возвращаем созданную карту
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(card); err != nil {
		h.logger.WithError(err).Error("Ошибка кодирования ответа")
	}
}

func (h *CardHandler) ListCards(w http.ResponseWriter, r *http.Request) {
	// Получаем ID пользователя из контекста
	userID, ok := r.Context().Value("userID").(string)
	if !ok {
		h.logger.Warn("Попытка получения списка карт без авторизации")
		http.Error(w, "Требуется авторизация", http.StatusUnauthorized)
		return
	}

	userUUID, err := uuid.Parse(userID)
	if err != nil {
		h.logger.WithField("userID", userID).Warn("Неверный формат ID пользователя")
		http.Error(w, "Неверный ID пользователя", http.StatusBadRequest)
		return
	}

	h.logger.WithField("userID", userUUID).Info("Запрос списка карт пользователя")

	// Получаем список карт через сервис
	cards, err := h.cardService.ListUserCards(r.Context(), userUUID)
	if err != nil {
		h.logger.WithError(err).Error("Ошибка получения списка карт")
		http.Error(w, "Ошибка получения карт", http.StatusInternalServerError)
		return
	}

	h.logger.WithField("count", len(cards)).Info("Успешно получен список карт")

	// Возвращаем список карт
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(cards); err != nil {
		h.logger.WithError(err).Error("Ошибка кодирования списка карт")
	}
}

func (h *CardHandler) GetCard(w http.ResponseWriter, r *http.Request) {
	// Получаем ID пользователя из контекста
	userID, ok := r.Context().Value("userID").(string)
	if !ok {
		h.logger.Warn("Попытка получения карты без авторизации")
		http.Error(w, "Требуется авторизация", http.StatusUnauthorized)
		return
	}

	userUUID, err := uuid.Parse(userID)
	if err != nil {
		h.logger.WithField("userID", userID).Warn("Неверный формат ID пользователя")
		http.Error(w, "Неверный ID пользователя", http.StatusBadRequest)
		return
	}

	// Получаем ID карты из URL
	vars := mux.Vars(r)
	cardID, err := uuid.Parse(vars["id"])
	if err != nil {
		h.logger.WithField("cardID", vars["id"]).Warn("Неверный формат ID карты")
		http.Error(w, "Неверный ID карты", http.StatusBadRequest)
		return
	}

	h.logger.WithFields(logrus.Fields{
		"userID": userUUID,
		"cardID": cardID,
	}).Info("Запрос информации о карте")

	// Получаем данные карты через сервис
	card, err := h.cardService.GetCard(r.Context(), cardID, userUUID)
	if err != nil {
		h.logger.WithError(err).Error("Ошибка получения карты")
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "Карта не найдена", http.StatusNotFound)
		} else {
			http.Error(w, "Ошибка получения карты", http.StatusInternalServerError)
		}
		return
	}

	h.logger.WithField("cardID", cardID).Info("Успешно получена информация о карте")

	// Возвращаем данные карты
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(card); err != nil {
		h.logger.WithError(err).Error("Ошибка кодирования данных карты")
	}
}

func (h *CardHandler) ProcessPayment(w http.ResponseWriter, r *http.Request) {
	// Получаем ID пользователя из контекста
	userID, ok := r.Context().Value("userID").(string)
	if !ok {
		h.logger.Warn("Попытка оплаты без авторизации")
		http.Error(w, "Требуется авторизация", http.StatusUnauthorized)
		return
	}

	userUUID, err := uuid.Parse(userID)
	if err != nil {
		h.logger.WithField("userID", userID).Warn("Неверный формат ID пользователя")
		http.Error(w, "Неверный ID пользователя", http.StatusBadRequest)
		return
	}

	// Декодируем запрос на оплату
	var req model.PaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.WithError(err).Warn("Ошибка декодирования запроса на оплату")
		http.Error(w, "Неверный формат запроса", http.StatusBadRequest)
		return
	}

	h.logger.WithFields(logrus.Fields{
		"userID": userUUID,
		"amount": req.Amount,
	}).Info("Попытка выполнения платежа")

	// Обрабатываем платеж через сервис
	paymentResponse, err := h.cardService.ProcessPayment(r.Context(), &req, userUUID)
	if err != nil {
		h.logger.WithError(err).Error("Ошибка обработки платежа")

		if paymentResponse != nil {
			h.logger.WithField("status", paymentResponse.Status).Warn("Платеж отклонен")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			if err := json.NewEncoder(w).Encode(paymentResponse); err != nil {
				h.logger.WithError(err).Error("Ошибка кодирования ответа платежа")
			}
			return
		}

		http.Error(w, "Ошибка платежа", http.StatusBadRequest)
		return
	}

	h.logger.WithField("status", paymentResponse.Status).Info("Платеж успешно обработан")

	// Возвращаем результат платежа
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if paymentResponse != nil {
		if err := json.NewEncoder(w).Encode(paymentResponse); err != nil {
			h.logger.WithError(err).Error("Ошибка кодирования ответа платежа")
		}
	}
}
