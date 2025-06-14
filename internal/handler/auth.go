package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	"banking-api/internal/model"
	"banking-api/internal/service"
)

// AuthHandler обрабатывает запросы аутентификации
type AuthHandler struct {
	authService *service.AuthService // Сервис аутентификации
	logger      *logrus.Logger       // Логгер
}

// NewAuthHandler создает новый AuthHandler
func NewAuthHandler(authService *service.AuthService, logger *logrus.Logger) *AuthHandler {
	return &AuthHandler{authService: authService, logger: logger}
}

// RegisterRoutes регистрирует маршруты для аутентификации
func (h *AuthHandler) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/signup", h.SignUp).Methods("POST") // Маршрут для регистрации
	router.HandleFunc("/signin", h.SignIn).Methods("POST") // Маршрут для входа
}

// SignUp обрабатывает запрос на регистрацию нового пользователя
func (h *AuthHandler) SignUp(w http.ResponseWriter, r *http.Request) {
	var input model.SignUpInput

	// Декодируем входные данные
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.logger.WithError(err).Error("Не удалось декодировать входные данные для регистрации")
		http.Error(w, "Неверный формат запроса", http.StatusBadRequest)
		return
	}

	// Проверяем валидность входных данных
	if err := input.Validate(); err != nil {
		h.logger.WithError(err).Error("Ошибка валидации входных данных для регистрации")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Регистрируем пользователя
	user, err := h.authService.SignUp(r.Context(), input)
	if err != nil {
		h.logger.WithError(err).Error("Не удалось зарегистрировать пользователя")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Формируем ответ
	response := map[string]interface{}{
		"id":         user.ID,
		"username":   user.Username,
		"email":      user.Email,
		"created_at": user.CreatedAt.Format(time.RFC3339),
	}

	// Устанавливаем заголовок и код ответа
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response) // Отправляем ответ
}

// SignIn обрабатывает запрос на вход пользователя
func (h *AuthHandler) SignIn(w http.ResponseWriter, r *http.Request) {
	var input model.SignInInput

	// Декодируем входные данные
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.logger.WithError(err).Error("Не удалось декодировать входные данные для входа")
		http.Error(w, "Неверный формат запроса", http.StatusBadRequest)
		return
	}

	// Выполняем вход пользователя
	token, err := h.authService.SignIn(r.Context(), input)
	if err != nil {
		h.logger.WithError(err).Error("Не удалось войти в систему")
		http.Error(w, "Неверные учетные данные", http.StatusUnauthorized)
		return
	}

	// Формируем ответ с токеном
	response := map[string]string{
		"token": token,
	}

	// Устанавливаем заголовок и код ответа
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response) // Отправляем ответ
}
