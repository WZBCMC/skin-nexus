package plugin

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// SecretBox encrypts plugin config secrets at rest with AES-256-GCM. The key
// comes from SKIN_PLUGIN_SECRET_KEY (hex or base64) or a generated 32-byte
// file next to the data directory. Losing the key only loses plugin secrets,
// never user data or textures.
type SecretBox struct {
	key        []byte
	keyVersion int
}

const secretKeyVersion = 1

func NewSecretBox(envValue, keyPath string) (*SecretBox, error) {
	if envValue != "" {
		key, err := decodeSecretKey(envValue)
		if err != nil {
			return nil, err
		}
		return &SecretBox{key: key, keyVersion: secretKeyVersion}, nil
	}
	if keyPath == "" {
		return nil, errors.New("plugin secret key path is empty")
	}
	key, err := os.ReadFile(keyPath)
	if err == nil {
		if len(key) != 32 {
			return nil, fmt.Errorf("plugin secret key %s must be 32 bytes (got %d)", keyPath, len(key))
		}
		return &SecretBox{key: key, keyVersion: secretKeyVersion}, nil
	}
	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read plugin secret key: %w", err)
	}
	key = make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate plugin secret key: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(keyPath), 0o700); err != nil {
		return nil, fmt.Errorf("create plugin secret dir: %w", err)
	}
	if err := os.WriteFile(keyPath, key, 0o600); err != nil {
		return nil, fmt.Errorf("write plugin secret key: %w", err)
	}
	return &SecretBox{key: key, keyVersion: secretKeyVersion}, nil
}

func decodeSecretKey(s string) ([]byte, error) {
	if b, err := hex.DecodeString(s); err == nil && len(b) == 32 {
		return b, nil
	}
	if b, err := base64.StdEncoding.DecodeString(s); err == nil && len(b) == 32 {
		return b, nil
	}
	return nil, errors.New("SKIN_PLUGIN_SECRET_KEY must be 32 bytes encoded as hex or base64")
}

func (b *SecretBox) KeyVersion() int { return b.keyVersion }

func (b *SecretBox) Seal(plaintext []byte) (ciphertext, nonce []byte, err error) {
	block, err := aes.NewCipher(b.key)
	if err != nil {
		return nil, nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}
	nonce = make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, err
	}
	return gcm.Seal(nil, nonce, plaintext, nil), nonce, nil
}

func (b *SecretBox) Open(ciphertext, nonce []byte) ([]byte, error) {
	block, err := aes.NewCipher(b.key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(nonce) != gcm.NonceSize() {
		return nil, errors.New("invalid secret nonce")
	}
	return gcm.Open(nil, nonce, ciphertext, nil)
}

func (b *SecretBox) SealString(s string) (ciphertext, nonce []byte, err error) {
	return b.Seal([]byte(s))
}

func (b *SecretBox) OpenString(ciphertext, nonce []byte) (string, error) {
	plain, err := b.Open(ciphertext, nonce)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
