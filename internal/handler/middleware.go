package handler

import (
	"context"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	"banking-api/internal/service"
)

// AuthMiddleware проверяет наличие и валидность JWT токена в заголовке Authorization
func AuthMiddleware(authService *service.AuthService, logger *logrus.Logger) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Получаем заголовок Authorization
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				logger.Error("Отсутствует заголовок Authorization")
				http.Error(w, "Заголовок Authorization обязателен", http.StatusUnauthorized)
				return
			}

			// Проверяем формат заголовка
			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				logger.Error("Неверный формат заголовка Authorization")
				http.Error(w, "Неверный формат заголовка Authorization", http.StatusUnauthorized)
				return
			}

			token := parts[1]
			// Парсим токен и проверяем его валидность
			userID, err := authService.ParseToken(token)
			if err != nil {
				logger.WithError(err).Error("Неверный токен")
				http.Error(w, "Неверный токен", http.StatusUnauthorized)
				return
			}

			// Добавляем userID в контекст
			ctx := context.WithValue(r.Context(), "userID", userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
