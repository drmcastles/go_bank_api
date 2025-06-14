package service

import (
	"crypto/tls"
	"fmt"
	"github.com/go-mail/mail/v2"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"os"
	"strconv"
	"time"
)

type EmailSender struct {
	dialer               *mail.Dialer
	logger               *logrus.Logger
	enabled              bool
	isInsecureSkipVerify bool
}

func NewEmailSender(logger *logrus.Logger) *EmailSender {
	smtpHost := os.Getenv("SMTP_HOST")
	smtpPortStr := os.Getenv("SMTP_PORT")
	smtpUser := os.Getenv("SMTP_USER")
	smtpPass := os.Getenv("SMTP_PASS")
	enabledStr := os.Getenv("EMAIL_SENDER_ENABLED")
	isInsecureSkipVerifyStr := os.Getenv("INSECURE_SKIP_VERIFY")
	// Преобразуем smtpPort в int
	smtpPort, err := strconv.Atoi(smtpPortStr)
	if err != nil {
		logger.Fatalf("Ошибка преобразования SMTP_PORT: %v", err)
	}
	// Преобразуем enabled в bool
	enabled := enabledStr == "true"
	isInsecureSkipVerify := isInsecureSkipVerifyStr == "true"
	d := mail.NewDialer(smtpHost, smtpPort, smtpUser, smtpPass)
	d.TLSConfig = &tls.Config{
		ServerName:         smtpHost,
		InsecureSkipVerify: isInsecureSkipVerify,
	}
	return &EmailSender{
		dialer:  d,
		logger:  logger,
		enabled: enabled,
	}
}

func (es *EmailSender) SendPaymentNotification(email string, amount float64, paymentType string) error {
	if !es.enabled {
		es.logger.Warn("Отправка уведомлений отключена")
		return nil
	}

	subject := fmt.Sprintf("Уведомление о платеже (%s)", paymentType)
	content := fmt.Sprintf(`
		<h1>Уведомление о платеже</h1>
		<p>Тип платежа: <strong>%s</strong></p>
		<p>Сумма: <strong>%.2f RUB</strong></p>
		<p>Дата: <strong>%s</strong></p>
		<small>Это автоматическое уведомление, пожалуйста, не отвечайте на него</small>
	`, paymentType, amount, time.Now().Format("02.01.2006 15:04"))

	return es.sendEmail(email, subject, content)
}

func (es *EmailSender) SendTransferNotification(email string, amount float64, from, to string) error {
	if !es.enabled {
		es.logger.Warn("Отправка уведомлений отключена")
		return nil
	}

	subject := "Уведомление о переводе средств"
	content := fmt.Sprintf(`
		<h1>Уведомление о переводе</h1>
		<p>Сумма перевода: <strong>%.2f RUB</strong></p>
		<p>Со счета: <strong>%s</strong></p>
		<p>На счет: <strong>%s</strong></p>
		<p>Дата: <strong>%s</strong></p>
		<small>Это автоматическое уведомление, пожалуйста, не отвечайте на него</small>
	`, amount, from, to, time.Now().Format("02.01.2006 15:04"))

	return es.sendEmail(email, subject, content)
}

func (es *EmailSender) SendCreditPaymentNotification(email string, amount float64, creditID uuid.UUID) error {
	if !es.enabled {
		es.logger.Warn("Отправка уведомлений отключена")
		return nil
	}

	subject := "Уведомление о платеже по кредиту"
	content := fmt.Sprintf(`
		<h1>Уведомление о платеже по кредиту</h1>
		<p>Номер кредита: <strong>%s</strong></p>
		<p>Сумма платежа: <strong>%.2f RUB</strong></p>
		<p>Дата: <strong>%s</strong></p>
		<small>Это автоматическое уведомление, пожалуйста, не отвечайте на него</small>
	`, creditID.String(), amount, time.Now().Format("02.01.2006 15:04"))

	return es.sendEmail(email, subject, content)
}

func (es *EmailSender) sendEmail(to, subject, body string) error {
	m := mail.NewMessage()
	m.SetHeader("From", os.Getenv("SMTP_USER"))
	m.SetHeader("To", to)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", body)

	if err := es.dialer.DialAndSend(m); err != nil {
		es.logger.WithError(err).Error("Ошибка отправки email")
		return fmt.Errorf("не удалось отправить email: %w", err)
	}

	es.logger.Infof("Email успешно отправлен на %s", to)
	return nil
}
