package service

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/beevik/etree"
	"github.com/sirupsen/logrus"
	"io"
	"net/http"
	"time"
)

type CBRClient struct {
	httpClient *http.Client
	logger     *logrus.Logger
}

// NewCBRClient создаёт новый экземпляр клиента для взаимодействия с веб-сервисом ЦБ РФ
func NewCBRClient(logger *logrus.Logger) *CBRClient {
	return &CBRClient{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger,
	}
}

// buildSOAPRequest формирует SOAP-запрос для получения ключевой ставки за последние 30 дней
func buildSOAPRequest() string {
	fromDate := time.Now().AddDate(0, 0, -30).Format("2006-01-02")
	toDate := time.Now().Format("2006-01-02")
	return fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
        <soap12:Envelope xmlns:soap12="http://www.w3.org/2003/05/soap-envelope">
            <soap12:Body>
                <KeyRate xmlns="http://web.cbr.ru/">
                    <fromDate>%s</fromDate>
                    <ToDate>%s</ToDate>
                </KeyRate>
            </soap12:Body>
        </soap12:Envelope>`, fromDate, toDate)
}

// sendRequest отправляет SOAP-запрос в ЦБ РФ и возвращает необработанный ответ
func sendRequest(soapRequest string) ([]byte, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(
		"POST",
		"https://www.cbr.ru/DailyInfoWebServ/DailyInfo.asmx",
		bytes.NewBuffer([]byte(soapRequest)),
	)
	if err != nil {
		return nil, err
	}

	// Установка заголовков
	req.Header.Set("Content-Type", "application/soap+xml; charset=utf-8")
	req.Header.Set("SOAPAction", "http://web.cbr.ru/KeyRate")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ошибка при выполнении HTTP-запроса: %v", err)
	}
	defer resp.Body.Close()

	// Чтение тела ответа
	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка при чтении ответа: %v", err)
	}

	return rawBody, nil
}

// parseXMLResponse парсит XML-ответ и извлекает значение ключевой ставки
func parseXMLResponse(rawBody []byte) (float64, error) {
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(rawBody); err != nil {
		return 0, fmt.Errorf("ошибка при разборе XML: %v", err)
	}

	// Поиск всех элементов ставки
	krElements := doc.FindElements("//diffgram/KeyRate/KR")
	if len(krElements) == 0 {
		return 0, errors.New("данные по ключевой ставке не найдены")
	}

	latestKR := krElements[0]
	rateElement := latestKR.FindElement("./Rate")
	if rateElement == nil {
		return 0, errors.New("элемент <Rate> отсутствует в XML-ответе")
	}

	rateStr := rateElement.Text()

	var rate float64
	// Преобразование строки в число
	if _, err := fmt.Sscanf(rateStr, "%f", &rate); err != nil {
		return 0, fmt.Errorf("ошибка при преобразовании ставки: %v", err)
	}

	return rate, nil
}

// GetCentralBankRate получает актуальную ключевую ставку из ЦБ РФ
func (c CBRClient) GetCentralBankRate() (float64, error) {
	c.logger.Info("Формирование SOAP-запроса к ЦБ РФ для получения ключевой ставки...")
	soapRequest := buildSOAPRequest()

	c.logger.Info("Отправка запроса в ЦБ РФ...")
	rawBody, err := sendRequest(soapRequest)
	if err != nil {
		c.logger.WithError(err).Error("Ошибка при отправке запроса в ЦБ РФ")
		return 0, err
	}
	c.logger.Debug("Ответ от ЦБ РФ успешно получен")

	c.logger.Info("Анализ XML-ответа от ЦБ РФ...")
	rate, err := parseXMLResponse(rawBody)
	if err != nil {
		c.logger.WithError(err).Error("Ошибка при разборе XML-ответа от ЦБ РФ")
		return 0, err
	}

	c.logger.WithField("key_rate", rate).Info("Ключевая ставка успешно получена")
	return rate, nil
}
