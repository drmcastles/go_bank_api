package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"

	"banking-api/internal/config"
	"banking-api/internal/crypto"
	"banking-api/internal/handler"
	"banking-api/internal/repository"
	"banking-api/internal/service"
)

func main() {
	logger := logrus.New()
	// Уровень логирования (Debug для разработки, Info для продакшена)
	logger.SetLevel(logrus.DebugLevel)
	logger.SetFormatter(&logrus.JSONFormatter{})

	// Загрузка конфигурации приложения
	cfg, err := config.LoadConfig()
	if err != nil {
		logger.Fatalf("Ошибка загрузки конфигурации: %v", err)
	}

	// Подключение к PostgreSQL
	db, err := sql.Open("postgres", fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName,
	))
	if err != nil {
		logger.Fatalf("Ошибка подключения к базе данных: %v", err)
	}
	defer db.Close()

	// Проверка соединения с БД
	if err := db.Ping(); err != nil {
		logger.Fatalf("Ошибка проверки соединения с БД: %v", err)
	}

	// Инициализация PGP для шифрования данных карт
	pgpManager, err := crypto.NewPGPManager("config/pgp-key.asc")
	if err != nil {
		logger.Fatalf("Ошибка инициализации PGP: %v", err)
	}

	pgpKey := pgpManager.GetEntity()
	hmacKey := []byte(os.Getenv("HMAC_SECRET"))
	if len(hmacKey) == 0 {
		logger.Fatal("Переменная окружения HMAC_SECRET не установлена")
	}
	if len(hmacKey) < 32 {
		logger.Fatal("HMAC ключ должен быть длиной минимум 32 байта")
	}

	// Инициализация репозиториев
	logger.Info("Инициализация репозиториев...")
	userRepo := repository.NewUserRepository(db, logger)
	accountRepo := repository.NewAccountRepository(db, logger)
	transactionRepo := repository.NewTransactionRepository(db, logger)
	cardRepo := repository.NewCardRepository(db, logger)
	creditRepo := repository.NewCreditRepository(db, logger)
	emailSender := service.NewEmailSender(logger)

	// Инициализация сервисов
	logger.Info("Инициализация сервисов...")
	authService := service.NewAuthService(userRepo, cfg.JWTSecret, cfg.TokenExpiry, logger)
	accountService := service.NewAccountService(userRepo, accountRepo, transactionRepo, emailSender, logger)
	cardService := service.NewCardService(userRepo, cardRepo, accountRepo, transactionRepo, emailSender, pgpKey, hmacKey, logger)
	cbrClient := service.NewCBRClient(logger)
	creditService := service.NewCreditService(
		userRepo,
		creditRepo,
		accountRepo,
		transactionRepo,
		emailSender,
		cbrClient,
		logger,
	)
	analyticsService := service.NewAnalyticService(
		transactionRepo,
		creditRepo,
		accountRepo,
		logger,
	)

	// Инициализация HTTP обработчиков
	logger.Info("Инициализация обработчиков API...")
	authHandler := handler.NewAuthHandler(authService, logger)
	accountHandler := handler.NewAccountHandler(accountService, logger)
	cardHandler := handler.NewCardHandler(cardService, logger)
	creditHandler := handler.NewCreditHandler(creditService, logger)
	analyticsHandler := handler.NewAnalyticsHandler(
		accountService,
		creditService,
		analyticsService,
		logger,
	)

	// Настройка маршрутизатора
	router := mux.NewRouter()

	// 1. Публичные маршруты для аутентификации
	publicRouter := router.PathPrefix("/auth").Subrouter()
	authHandler.RegisterRoutes(publicRouter) // Регистрация /signup и /signin

	// 2. Защищенные API маршруты (требуется JWT токен)
	apiRouter := router.PathPrefix("/api").Subrouter()
	apiRouter.Use(handler.AuthMiddleware(authService, logger))

	// Маршруты для работы со счетами
	accountRouter := apiRouter.PathPrefix("/accounts").Subrouter()
	accountHandler.RegisterRoutes(accountRouter)

	// Маршруты для работы с картами
	cardRouter := apiRouter.PathPrefix("/cards").Subrouter()
	cardHandler.RegisterRoutes(cardRouter)

	// Маршруты для работы с кредитами
	creditRouter := apiRouter.PathPrefix("/credits").Subrouter()
	creditHandler.RegisterRoutes(creditRouter)

	analyticsRouter := apiRouter.PathPrefix("/analytics").Subrouter()
	analyticsHandler.RegisterRoutes(analyticsRouter)

	// Настройка планировщика для автоматической обработки платежей
	logger.Info("Настройка планировщика обработки платежей...")
	c := cron.New()
	_, err = c.AddFunc("0 */12 * * *", func() {
		logger.Info("Запуск автоматической обработки платежей по кредитам")
		if err := creditService.ProcessPayments(context.Background()); err != nil {
			logger.WithError(err).Error("Ошибка обработки платежей")
		} else {
			logger.Info("Автоматическая обработка платежей завершена успешно")
		}
	})
	if err != nil {
		logger.Fatalf("Ошибка настройки планировщика: %v", err)
	}
	c.Start()

	// Настройка и запуск HTTP сервера
	server := &http.Server{
		Addr:    ":8080",
		Handler: router,
	}

	go func() {
		logger.Info("Запуск сервера на порту :8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("Ошибка сервера: %v", err)
		}
	}()

	// Ожидание сигналов для graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Завершение работы сервера...")
	if err := server.Shutdown(context.Background()); err != nil {
		logger.Errorf("Ошибка при завершении работы сервера: %v", err)
	}
	logger.Info("Сервер успешно остановлен")
}
