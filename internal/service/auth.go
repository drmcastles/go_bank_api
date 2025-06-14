package service

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"

	"banking-api/internal/model"
	"banking-api/internal/repository"
)

type AuthService struct {
	userRepo    *repository.UserRepository
	jwtSecret   string
	tokenExpiry time.Duration
	logger      *logrus.Logger
}

func NewAuthService(userRepo *repository.UserRepository, jwtSecret string, tokenExpiry time.Duration, logger *logrus.Logger) *AuthService {
	return &AuthService{
		userRepo:    userRepo,
		jwtSecret:   jwtSecret,
		tokenExpiry: tokenExpiry,
		logger:      logger,
	}
}

// SignUp Регистрация нового пользователя
func (s *AuthService) SignUp(ctx context.Context, input model.SignUpInput) (*model.User, error) {
	s.logger.WithFields(logrus.Fields{
		"email":    input.Email,
		"username": input.Username,
	}).Info("Попытка регистрации нового пользователя")

	// Проверка на существование пользователя
	exists, err := s.userRepo.ExistsByEmailOrUsername(ctx, input.Email, input.Username)
	if err != nil {
		s.logger.WithError(err).Error("Не удалось проверить существование пользователя")
		return nil, fmt.Errorf("ошибка проверки существования пользователя: %w", err)
	}
	if exists {
		s.logger.Warn("Пользователь с таким email или username уже существует")
		return nil, fmt.Errorf("пользователь с таким email или username уже существует")
	}

	// Хеширование пароля
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		s.logger.WithError(err).Error("Не удалось захешировать пароль")
		return nil, fmt.Errorf("ошибка хеширования пароля: %w", err)
	}

	// Создание пользователя
	now := time.Now()
	user := &model.User{
		ID:        uuid.New(),
		Username:  input.Username,
		Email:     input.Email,
		Password:  string(hashedPassword),
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		s.logger.WithError(err).Error("Не удалось создать пользователя в базе данных")
		return nil, fmt.Errorf("ошибка создания пользователя: %w", err)
	}

	s.logger.WithField("user_id", user.ID).Info("Пользователь успешно зарегистрирован")
	return user, nil
}

// SignIn Авторизация пользователя и генерация JWT токена
func (s *AuthService) SignIn(ctx context.Context, input model.SignInInput) (string, error) {
	s.logger.WithField("email", input.Email).Info("Попытка входа пользователя")

	// Поиск пользователя по email
	user, err := s.userRepo.FindByEmail(ctx, input.Email)
	if err != nil {
		s.logger.WithError(err).Warn("Пользователь не найден или неверные учётные данные")
		return "", fmt.Errorf("неверные учетные данные")
	}

	// Проверка пароля
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Password)); err != nil {
		s.logger.Warn("Неверный пароль при попытке входа")
		return "", fmt.Errorf("неверные учетные данные")
	}

	// Генерация JWT токена
	token, err := s.GenerateJWTToken(user.ID.String())
	if err != nil {
		s.logger.WithError(err).Error("Не удалось сгенерировать JWT токен")
		return "", fmt.Errorf("ошибка генерации токена: %w", err)
	}

	s.logger.WithField("user_id", user.ID).Info("Пользователь успешно вошёл в систему")
	return token, nil
}

// GenerateJWTToken Генерация JWT токена
func (s *AuthService) GenerateJWTToken(userID string) (string, error) {
	claims := jwt.RegisteredClaims{
		Subject:   userID,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.tokenExpiry)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.jwtSecret))
}

// ParseToken Разбор и валидация JWT токена
func (s *AuthService) ParseToken(tokenString string) (string, error) {
	s.logger.Debug("Попытка парсинга JWT токена")

	claims := &jwt.RegisteredClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		// Проверка метода подписи
		return []byte(s.jwtSecret), nil
	})

	if err != nil || !token.Valid {
		s.logger.WithError(err).Warn("Невалидный JWT токен")
		return "", fmt.Errorf("невалидный токен: %w", err)
	}

	// Извлечение ID пользователя
	userID := claims.Subject
	if userID == "" {
		s.logger.Error("Не удалось извлечь идентификатор пользователя из токена")
		return "", fmt.Errorf("некорректные claims токена")
	}

	s.logger.WithField("user_id", userID).Info("JWT токен успешно распознан")
	return userID, nil
}
