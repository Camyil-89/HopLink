package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"golang.org/x/crypto/pbkdf2"
)

const (
	Format     = "hoplink-encrypted-settings"
	Version    = 1
	iterations = 310000
)

type Envelope struct {
	Format     string `json:"format"`
	Version    int    `json:"version"`
	Salt       string `json:"salt"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

func DeriveKey(password string, salt []byte) []byte {
	return pbkdf2.Key([]byte(password), salt, iterations, 32, sha256.New)
}

func Encrypt(plaintext, key, salt []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	sealed := aead.Seal(nil, nonce, plaintext, []byte(Format))
	envelope := Envelope{Format: Format, Version: Version, Salt: base64.RawStdEncoding.EncodeToString(salt), Nonce: base64.RawStdEncoding.EncodeToString(nonce), Ciphertext: base64.RawStdEncoding.EncodeToString(sealed)}
	return json.MarshalIndent(envelope, "", "  ")
}

func Decrypt(data []byte, password string) ([]byte, []byte, []byte, error) {
	var envelope Envelope
	if err := json.Unmarshal(data, &envelope); err != nil || envelope.Format != Format || envelope.Version != Version {
		return nil, nil, nil, fmt.Errorf("файл настроек повреждён или имеет неизвестный формат")
	}
	salt, err := base64.RawStdEncoding.DecodeString(envelope.Salt)
	if err != nil || len(salt) < 16 {
		return nil, nil, nil, fmt.Errorf("неверный формат зашифрованных настроек")
	}
	nonce, err := base64.RawStdEncoding.DecodeString(envelope.Nonce)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("неверный формат зашифрованных настроек")
	}
	ciphertext, err := base64.RawStdEncoding.DecodeString(envelope.Ciphertext)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("неверный формат зашифрованных настроек")
	}
	key := DeriveKey(password, salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, nil, err
	}
	plaintext, err := aead.Open(nil, nonce, ciphertext, []byte(Format))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("неверный пароль")
	}
	return plaintext, key, salt, nil
}
