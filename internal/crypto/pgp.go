package crypto

import (
	"crypto"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/armor"
	"golang.org/x/crypto/openpgp/packet"
)

type PGPManager struct {
	entity  *openpgp.Entity // PGP сущность
	keyPath string          // Путь к файлу ключа
}

// NewPGPManager создает новый менеджер PGP ключей
func NewPGPManager(keyPath string) (*PGPManager, error) {
	manager := &PGPManager{keyPath: keyPath}

	if err := manager.init(); err != nil {
		return nil, fmt.Errorf("не удалось инициализировать PGP: %w", err)
	}

	return manager, nil
}

// init инициализирует PGP менеджер, загружая существующий ключ или создавая новый
func (m *PGPManager) init() error {
	// Попытка загрузить существующий ключ
	if _, err := os.Stat(m.keyPath); err == nil {
		entity, err := m.loadKeyFromFile()
		if err != nil {
			return fmt.Errorf("не удалось загрузить PGP ключ: %w", err)
		}
		m.entity = entity
		return nil
	}

	// Генерация нового ключа
	return m.generateAndSaveKey()
}

// generateAndSaveKey генерирует новый PGP ключ и сохраняет его в файл
func (m *PGPManager) generateAndSaveKey() error {
	config := &packet.Config{
		Rand:          rand.Reader,
		RSABits:       4096,
		DefaultHash:   crypto.SHA256,
		DefaultCipher: packet.CipherAES256,
	}

	entity, err := openpgp.NewEntity(
		"Banking API Server",
		"",
		"banking-api@yourdomain.com",
		config,
	)
	if err != nil {
		return fmt.Errorf("не удалось сгенерировать сущность: %w", err)
	}

	// Подписываем идентификаторы
	for _, id := range entity.Identities {
		err := id.SelfSignature.SignUserId(
			id.UserId.Id,
			entity.PrimaryKey,
			entity.PrivateKey,
			config,
		)
		if err != nil {
			return fmt.Errorf("не удалось подписать идентичность: %w", err)
		}
	}

	// Сохраняем ключ
	if err := os.MkdirAll(filepath.Dir(m.keyPath), 0700); err != nil {
		return fmt.Errorf("не удалось создать директорию для ключа: %w", err)
	}

	file, err := os.OpenFile(m.keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("не удалось создать файл ключа: %w", err)
	}
	defer file.Close()

	armorWriter, err := armor.Encode(file, openpgp.PrivateKeyType, nil)
	if err != nil {
		return fmt.Errorf("не удалось создать armor writer: %w", err)
	}

	if err := entity.SerializePrivate(armorWriter, config); err != nil {
		armorWriter.Close()
		return fmt.Errorf("не удалось сериализовать приватный ключ: %w", err)
	}

	if err := armorWriter.Close(); err != nil {
		return fmt.Errorf("не удалось закрыть armor writer: %w", err)
	}

	m.entity = entity
	return nil
}

// GetEntity возвращает PGP сущность
func (m *PGPManager) GetEntity() *openpgp.Entity {
	return m.entity
}

// loadKeyFromFile загружает ключ из файла
func (m *PGPManager) loadKeyFromFile() (*openpgp.Entity, error) {
	file, err := os.Open(m.keyPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	block, err := armor.Decode(file)
	if err != nil {
		return nil, err
	}

	if block.Type != openpgp.PrivateKeyType {
		return nil, errors.New("файл не является приватным ключом")
	}

	return openpgp.ReadEntity(packet.NewReader(block.Body))
}
