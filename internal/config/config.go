package config

import (
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
	"os"
	"time"
)

// Config содержит настройки приложения
type Config struct {
	DBHost      string        // Хост базы данных
	DBPort      string        // Порт базы данных
	DBUser      string        // Пользователь базы данных
	DBPassword  string        // Пароль базы данных
	DBName      string        // Имя базы данных
	JWTSecret   string        // Секрет для JWT
	TokenExpiry time.Duration // Время жизни токена
}

// LoadConfig загружает конфигурацию из .env файла
func LoadConfig() (*Config, error) {
	// Загружаем переменные окружения из .env файла
	if err := godotenv.Load(); err != nil {
		logrus.Warn("Файл .env не найден")
	}

	// Парсим время жизни токена
	expiry, err := time.ParseDuration(os.Getenv("TOKEN_EXPIRY"))
	if err != nil {
		expiry = 24 * time.Hour // По умолчанию 24 часа
	}

	// Создаем объект конфигурации
	config := &Config{
		DBHost:      getEnv("DB_HOST", "localhost"),
		DBPort:      getEnv("DB_PORT", "5432"),
		DBUser:      getEnv("DB_USER", "postgres"),
		DBPassword:  getEnv("DB_PASSWORD", "postgres"),
		DBName:      getEnv("DB_NAME", "auth_service"),
		JWTSecret:   getEnv("JWT_SECRET", "default-secret-key"),
		TokenExpiry: expiry,
	}

	return config, nil
}

// getEnv получает значение переменной окружения или возвращает значение по умолчанию
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
