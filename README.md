# Banking API

Описание проекта  
Данный проект – это REST API для банковского сервиса, предоставляющий функционал для регистрации и аутентификации пользователей, управления счетами, картами, кредитами, а также аналитикой финансовых операций. Кроме того, API интегрируется с внешними сервисами, такими как Центральный банк РФ и SMTP-сервер для рассылки уведомлений.

Основные возможности  
– Регистрация пользователей с проверкой уникальности данных  
– JWT-аутентификация  
– Создание и управление банковскими счетами  
– Работа с картами: выпуск, просмотр, оплата  
– Переводы между счетами и пополнение баланса  
– Оформление кредитов и управление графиком платежей  
– Аналитика по финансовым операциям  
– Интеграция с внешними сервисами (ЦБ РФ, SMTP)

Используемые технологии  
– Язык: Go (версия 1.23 и выше)  
– Маршрутизация: gorilla/mux  
– База данных: PostgreSQL с драйвером lib/pq  
– Аутентификация: JWT (github.com/golang-jwt/jwt/v5)  
– Логирование: logrus  
– Шифрование и безопасность: bcrypt, HMAC-SHA256, PGP  
– Отправка email: gomail.v2  
– Парсинг XML: beevik/etree

Архитектура приложения  
– Модели – структуры данных, валидация и (де)сериализация для API  
– Репозитории – SQL-запросы, обработка ошибок и транзакций  
– Сервисы – бизнес-логика и интеграция с внешними API  
– Обработчики (Handlers) – валидация запросов, вызов сервисов, формирование ответов  
– Маршруты – публичные и защищённые эндпоинты  
– Middleware – проверка JWT, добавление контекста, блокировка неавторизованных

Эндпоинты

Публичные  
– POST /auth/register – регистрация пользователя  
– POST /auth/login – вход в систему  

Защищённые (требуется JWT)  
– POST /api/accounts – создание банковского счета  
– POST /api/cards – выпуск карты  
– POST /api/transfer – перевод средств  
– GET /api/analytics – получение аналитики  
– GET /api/credits/{creditId}/schedule – график платежей по кредиту  
– GET /api/accounts/{accountId}/predict – прогноз баланса счета  

Безопасность  
– Номера и сроки действия карт шифруются с помощью PGP  
– CVV хранится в виде bcrypt-хеша  
– Проверка целостности данных выполняется через HMAC  
– JWT используется для аутентификации (секретный ключ задаётся через переменную окружения JWT_SECRET)  
– Пароли пользователей надёжно хешируются с bcrypt  

Дополнительные возможности  
– Планировщик задач (шедулер) для обработки просроченных платежей каждые 12 часов  
– Интеграция с ЦБ РФ через SOAP для получения ключевой ставки  
– Логирование всех ключевых операций с помощью logrus

Запуск проекта  
Клонирование репозитория  
cd banking-api  

Установка зависимостей  
go mod init banking-api  
go get github.com/gorilla/mux  
go get github.com/lib/pq  
go get github.com/golang-jwt/jwt/v5  
go get github.com/sirupsen/logrus  
go get golang.org/x/crypto/bcrypt  
go get github.com/joho/godotenv  
go get github.com/google/uuid  
go get -u golang.org/x/crypto/openpgp  
go get github.com/robfig/cron/v3  
go get github.com/beevik/etree  
go mod tidy  

Запуск базы данных  
docker run --name bank-pg -e POSTGRES_USER=user -e POSTGRES_PASSWORD=secret -e POSTGRES_DB=banking -p 5433:5432 -d postgres:latest  

Создание таблиц  
Необходимо применить миграции для создания следующих таблиц:  
– users – данные пользователей (001_init_users.up.sql)  
– accounts – банковские счета (002_add_accounts.up.sql)  
– cards – данные карт (003_add_cards_table.up.sql)  
– transactions – история операций (004_add_transactions.up.sql)  
– credits – кредиты (005_add_credits_table.up.sql)  
– payment_schedules – график платежей (006_add_payment_schedules_table.up.sql)  

Конфигурация окружения  
Создайте файл .env со следующими переменными:  

DB_HOST=localhost  
DB_PORT=5433  
DB_USER=user  
DB_PASSWORD=secret  
DB_NAME=banking  
JWT_SECRET=$(openssl rand -hex 32)  
TOKEN_EXPIRY=24h  
HMAC_SECRET=$(openssl rand -hex 32)  

SMTP_HOST=smtp.example.com  
SMTP_PORT=587  
SMTP_USER=noreply@example.com  
SMTP_PASS=strong_password  
INSECURE_SKIP_VERIFY=false  

EMAIL_SENDER_ENABLED=false  

Запуск приложения  
go run cmd/server/main.go  

Тестирование  
Для тестирования API используйте Postman коллекцию: Banking API.postman_collection.json
