package service

import (
	"bytes"
	"context"
	"crypto"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/openpgp/armor"
	"golang.org/x/crypto/openpgp/packet"
	"io"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/crypto/openpgp"

	"banking-api/internal/model"
	"banking-api/internal/repository"
)

type CardService struct {
	userRepo        *repository.UserRepository
	cardRepo        *repository.CardRepository
	accountRepo     *repository.AccountRepository
	transactionRepo *repository.TransactionRepository
	emailSender     *EmailSender
	pgpKey          *openpgp.Entity
	hmacKey         []byte
	logger          *logrus.Logger
}

func NewCardService(
	userRepo *repository.UserRepository,
	cardRepo *repository.CardRepository,
	accountRepo *repository.AccountRepository,
	transactionRepo *repository.TransactionRepository,
	emailSender *EmailSender,
	pgpKey *openpgp.Entity,
	hmacKey []byte,
	logger *logrus.Logger,
) *CardService {
	return &CardService{
		userRepo:        userRepo,
		cardRepo:        cardRepo,
		accountRepo:     accountRepo,
		transactionRepo: transactionRepo,
		emailSender:     emailSender,
		pgpKey:          pgpKey,
		hmacKey:         hmacKey,
		logger:          logger,
	}
}

func (s *CardService) CreateCard(ctx context.Context, userID uuid.UUID, req *model.CardRequest) (*model.CardResponse, error) {
	s.logger.Info("Создание новой карты...")

	// 1. Проверяем, что указанный счет принадлежит пользователю
	s.logger.WithFields(logrus.Fields{
		"user_id":    userID,
		"account_id": req.AccountID,
	}).Info("Проверка прав доступа к счёту пользователя")

	// Check if the account belongs to the user
	account, err := s.accountRepo.GetByID(ctx, req.AccountID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.logger.Warn("Счёт не найден")
			return nil, fmt.Errorf("счёт не найден")
		}
		s.logger.WithError(err).Error("Ошибка при получении счёта")
		return nil, fmt.Errorf("не удалось проверить счет: %w", err)
	}

	if account.UserID != userID {
		s.logger.Warn("Счёт не принадлежит пользователю")
		return nil, fmt.Errorf("счёт не принадлежит пользователю")
	}

	// 2. Генерация данных карты
	s.logger.Info("Генерация номера карты, срока действия и CVV")
	cardNumber := s.generateCardNumber()
	expiry := time.Now().Add(3 * 365 * 24 * time.Hour)
	expiryStr := expiry.Format("01/06")
	cvv := fmt.Sprintf("%03d", rand.Intn(1000))

	// 3. Шифрование данных
	s.logger.Debug("Шифрование данных карты")
	cardData := fmt.Sprintf("%s|%s", cardNumber, expiryStr)
	encryptedData, err := s.encryptData(cardData)
	if err != nil {
		s.logger.WithError(err).Error("Ошибка при шифровании данных карты")
		return nil, err
	}

	// 4. HMAC для целостности
	s.logger.Debug("Генерация HMAC для проверки целостности данных")
	h := hmac.New(sha256.New, s.hmacKey)
	h.Write([]byte(cardData))
	hmacValue := fmt.Sprintf("%x", h.Sum(nil))

	// 5. Хеширование CVV
	s.logger.Debug("Хеширование CVV-кода")
	cvvHash, err := bcrypt.GenerateFromPassword([]byte(cvv), bcrypt.DefaultCost)
	if err != nil {
		s.logger.WithError(err).Error("Ошибка при хешировании CVV")
		return nil, err
	}

	// 6. Сохранение в базу данных
	s.logger.Info("Сохранение карты в базу данных")
	card := &model.Card{
		ID:            uuid.New(),
		UserID:        userID,
		AccountID:     req.AccountID,
		Name:          req.Name,
		EncryptedData: string(encryptedData),
		CVVHash:       string(cvvHash),
		HMAC:          hmacValue,
		CreatedAt:     time.Now(),
		LastUsedAt:    time.Now(),
	}

	if err := s.cardRepo.Create(ctx, card); err != nil {
		s.logger.WithError(err).Error("Ошибка при сохранении карты")
		return nil, err
	}

	// 7. Проверка HMAC после создания карты
	if valid, err := s.verifyHMAC(card); err != nil || !valid {
		s.logger.WithFields(logrus.Fields{
			"error": err,
			"valid": valid,
		}).Error("Проверка HMAC не прошла после создания карты")
	}

	// 8. Ответ пользователю
	s.logger.Info("Карта успешно создана")
	return &model.CardResponse{
		ID:           card.ID,
		MaskedNumber: maskCardNumber(cardNumber),
		Expiry:       expiryStr,
		Name:         req.Name,
	}, nil
}

func (s *CardService) GetCard(ctx context.Context, cardID, userID uuid.UUID) (*model.CardResponse, error) {
	s.logger.WithFields(logrus.Fields{
		"card_id": cardID,
		"user_id": userID,
	}).Info("Получение информации о карте")

	card, err := s.cardRepo.GetByIDAndUser(ctx, cardID, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.logger.Warn("Карта не найдена")
			return nil, fmt.Errorf("карта не найдена")
		}
		s.logger.WithError(err).Error("Ошибка при получении карты")
		return nil, fmt.Errorf("не удалось получить карту: %w", err)
	}

	valid, err := s.verifyHMAC(card)
	if err != nil {
		s.logger.WithError(err).Error("Ошибка при проверке целостности карты")
		return nil, fmt.Errorf("не удалось проверить целостность карты: %w", err)
	}
	if !valid {
		s.logger.Error("Проверка целостности данных карты не пройдена")
		return nil, fmt.Errorf("проверка целостности данных не пройдена")
	}

	decryptedData, err := s.decryptCardData(card.EncryptedData)
	if err != nil {
		s.logger.WithError(err).Error("Ошибка при расшифровке данных карты")
		return nil, fmt.Errorf("не удалось расшифровать данные карты: %w", err)
	}

	return &model.CardResponse{
		ID:           card.ID,
		MaskedNumber: maskCardNumber(decryptedData.Number),
		Expiry:       decryptedData.Expiry,
		Name:         card.Name,
	}, nil
}

func (s *CardService) ListUserCards(ctx context.Context, userID uuid.UUID) ([]model.CardResponse, error) {
	s.logger.WithField("user_id", userID).Info("Получение списка карт пользователя")

	cards, err := s.cardRepo.ListByUser(ctx, userID)
	if err != nil {
		s.logger.WithError(err).Error("Ошибка при получении карт пользователя")
		return nil, fmt.Errorf("не удалось получить карты пользователя: %w", err)
	}

	var responses []model.CardResponse
	for _, card := range cards {
		valid, err := s.verifyHMAC(&card)
		if err != nil {
			s.logger.WithError(err).Errorf("Ошибка HMAC для карты %s", card.ID)
			return nil, fmt.Errorf("ошибка проверки целостности для карты %s: %w", card.ID, err)
		}
		if !valid {
			s.logger.Errorf("Нарушение целостности данных карты %s", card.ID)
			return nil, fmt.Errorf("проверка целостности не пройдена для карты %s", card.ID)
		}

		decryptedData, err := s.decryptCardData(card.EncryptedData)
		if err != nil {
			s.logger.WithError(err).Errorf("Ошибка расшифровки данных карты %s", card.ID)
			return nil, fmt.Errorf("ошибка расшифровки карты %s: %w", card.ID, err)
		}

		responses = append(responses, model.CardResponse{
			ID:           card.ID,
			MaskedNumber: maskCardNumber(decryptedData.Number),
			Expiry:       decryptedData.Expiry,
			Name:         card.Name,
		})
	}

	return responses, nil
}

func (s *CardService) verifyHMAC(card *model.Card) (bool, error) {
	decryptedData, err := s.decryptCardData(card.EncryptedData)
	if err != nil {
		return false, fmt.Errorf("не удалось расшифровать данные карты: %w", err)
	}

	cardData := fmt.Sprintf("%s|%s", decryptedData.Number, decryptedData.Expiry)

	h := hmac.New(sha256.New, s.hmacKey)
	h.Write([]byte(cardData))
	expectedMAC := fmt.Sprintf("%x", h.Sum(nil))

	s.logger.WithFields(logrus.Fields{
		"ожидаемый_hmac":   expectedMAC,
		"фактический_hmac": card.HMAC,
		"данные_карты":     cardData,
	}).Debug("Проверка HMAC")

	return hmac.Equal([]byte(card.HMAC), []byte(expectedMAC)), nil
}

func (s *CardService) decryptCardData(encrypted string) (*model.CardData, error) {
	block, err := armor.Decode(strings.NewReader(encrypted))
	if err != nil {
		return nil, fmt.Errorf("не удалось декодировать armor: %w", err)
	}

	md, err := openpgp.ReadMessage(block.Body, openpgp.EntityList{s.pgpKey}, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("ошибка расшифровки: %w", err)
	}

	plaintext, err := io.ReadAll(md.UnverifiedBody)
	if err != nil {
		return nil, fmt.Errorf("не удалось прочитать расшифрованные данные: %w", err)
	}

	parts := strings.Split(string(plaintext), "|")
	if len(parts) != 2 {
		return nil, fmt.Errorf("неверный формат данных карты")
	}

	return &model.CardData{
		Number: parts[0],
		Expiry: parts[1],
	}, nil
}

func (s *CardService) ProcessPayment(ctx context.Context, payment *model.PaymentRequest, userID uuid.UUID) (*model.PaymentResponse, error) {
	s.logger.WithFields(logrus.Fields{
		"card_id": payment.CardID,
		"user_id": userID,
		"amount":  payment.Amount,
	}).Info("Начало обработки платежа")

	if payment.Amount <= 0 {
		s.logger.Warn("Сумма платежа должна быть положительной")
		return nil, fmt.Errorf("сумма должна быть положительной")
	}

	card, err := s.cardRepo.GetByIDAndUser(ctx, payment.CardID, userID)
	if err != nil {
		s.logger.WithError(err).Error("Не удалось найти карту или получить доступ")
		return nil, fmt.Errorf("карта не найдена или доступ запрещён: %w", err)
	}

	valid, err := s.verifyHMAC(card)
	if err != nil {
		s.logger.WithError(err).Error("Ошибка проверки целостности данных карты")
		return nil, fmt.Errorf("ошибка проверки целостности карты: %w", err)
	}
	if !valid {
		s.logger.WithField("card_id", card.ID).Error("Проверка целостности HMAC не пройдена")
		return nil, fmt.Errorf("целостность данных нарушена")
	}

	decryptedData, err := s.decryptCardData(card.EncryptedData)
	if err != nil {
		s.logger.WithError(err).Error("Ошибка при расшифровке данных карты")
		return nil, fmt.Errorf("не удалось расшифровать данные карты: %w", err)
	}

	if card.AccountID == uuid.Nil {
		return nil, fmt.Errorf("карта не привязана к счёту")
	}

	paymentID := uuid.New()
	paymentResponse := &model.PaymentResponse{
		PaymentID:   paymentID,
		CardID:      card.ID,
		AccountID:   card.AccountID,
		Amount:      payment.Amount,
		Status:      "pending",
		ProcessedAt: time.Now(),
	}

	s.logger.WithFields(logrus.Fields{
		"masked_card": maskCardNumber(decryptedData.Number),
		"amount":      payment.Amount,
	}).Info("Платёж выполняется...")

	// Начинаем транзакцию
	tx, err := s.accountRepo.GetDB().BeginTx(ctx, nil)
	if err != nil {
		paymentResponse.Status = "failed"
		s.logger.WithError(err).Error("Не удалось начать транзакцию для списания средств")
		return paymentResponse, fmt.Errorf("ошибка транзакции: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// 1. Списание средств со счета
	if err := s.accountRepo.UpdateBalanceTx(ctx, tx, card.AccountID, -payment.Amount); err != nil {
		paymentResponse.Status = "failed"
		s.logger.WithError(err).Error("Ошибка при списании средств")
		return paymentResponse, fmt.Errorf("не удалось выполнить платёж: %w", err)
	}

	// 2. Создание записи о транзакции
	transaction := &model.Transaction{
		ID:              paymentID,
		AccountID:       card.AccountID,
		Amount:          payment.Amount,
		TransactionType: model.TransactionTypeCardPayment,
		ReferenceID:     &card.ID,
		CreatedAt:       time.Now(),
	}
	if err := s.transactionRepo.CreateTx(ctx, tx, transaction); err != nil {
		paymentResponse.Status = "failed"
		s.logger.WithError(err).Error("Ошибка при создании транзакции")
		return paymentResponse, fmt.Errorf("не удалось создать транзакцию: %w", err)
	}

	// Коммитим транзакцию
	if err := tx.Commit(); err != nil {
		paymentResponse.Status = "failed"
		s.logger.WithError(err).Error("Ошибка при фиксации транзакции")
		return paymentResponse, fmt.Errorf("не удалось выполнить платеж: %w", err)
	}

	paymentResponse.Status = "completed"
	if err := s.cardRepo.UpdateLastUsed(ctx, card.ID); err != nil {
		s.logger.WithError(err).Warn("Не удалось обновить дату последнего использования карты")
	}

	s.logger.Info("Платёж успешно завершён")

	// Отправка email уведомления
	if paymentResponse.Status == "completed" {
		// Получаем email пользователя (нужно добавить метод в UserRepository)
		user, err := s.userRepo.GetByID(ctx, userID)
		if err == nil && user.Email != "" {
			go func() {
				if err := s.emailSender.SendPaymentNotification(
					user.Email,
					payment.Amount,
					"оплата картой",
				); err != nil {
					s.logger.WithError(err).Warn("Не удалось отправить email уведомление")
				}
			}()
		}
	}
	return paymentResponse, nil
}

func (s *CardService) generateCardNumber() string {
	prefix := "4"
	for i := 0; i < 14; i++ {
		prefix += strconv.Itoa(rand.Intn(10))
	}

	sum := 0
	isSecondDigit := false
	for i := len(prefix) - 1; i >= 0; i-- {
		digit, _ := strconv.Atoi(string(prefix[i]))
		if isSecondDigit {
			digit *= 2
			if digit > 9 {
				digit = digit%10 + digit/10
			}
		}
		sum += digit
		isSecondDigit = !isSecondDigit
	}

	checkDigit := (10 - (sum % 10)) % 10
	return prefix + strconv.Itoa(checkDigit)
}

func (s *CardService) encryptData(data string) ([]byte, error) {
	buf := new(bytes.Buffer)

	armorWriter, err := armor.Encode(buf, "PGP MESSAGE", nil)
	if err != nil {
		return nil, fmt.Errorf("не удалось создать armor writer: %w", err)
	}

	config := &packet.Config{
		DefaultHash:            crypto.SHA256,
		DefaultCipher:          packet.CipherAES256,
		DefaultCompressionAlgo: packet.CompressionZLIB,
	}

	plaintextWriter, err := openpgp.Encrypt(armorWriter, []*openpgp.Entity{s.pgpKey}, nil, nil, config)
	if err != nil {
		armorWriter.Close()
		return nil, fmt.Errorf("не удалось создать writer для шифрования: %w", err)
	}

	if _, err := plaintextWriter.Write([]byte(data)); err != nil {
		armorWriter.Close()
		return nil, fmt.Errorf("ошибка при записи открытого текста: %w", err)
	}

	if err := plaintextWriter.Close(); err != nil {
		armorWriter.Close()
		return nil, fmt.Errorf("ошибка при закрытии writer текста: %w", err)
	}

	if err := armorWriter.Close(); err != nil {
		return nil, fmt.Errorf("ошибка при закрытии armor writer: %w", err)
	}

	return buf.Bytes(), nil
}

func maskCardNumber(number string) string {
	if len(number) < 4 {
		return "****"
	}
	return "**** **** **** " + number[len(number)-4:]
}
