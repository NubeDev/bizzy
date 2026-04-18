package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// masterKey is 32 bytes for AES-256.
type masterKey [32]byte

// LoadOrCreateKey reads the master key from BIZZY_SECRET_KEY env var,
// or from dataDir/.secret-key on disk. If neither exists, it generates
// a new random key and saves it to disk.
func LoadOrCreateKey(dataDir string) (masterKey, error) {
	// 1. Check env var (hex-encoded or raw 32 bytes).
	if env := os.Getenv("BIZZY_SECRET_KEY"); len(env) == 32 {
		var k masterKey
		copy(k[:], env)
		return k, nil
	}

	// 2. Check file on disk.
	keyPath := filepath.Join(dataDir, ".secret-key")
	data, err := os.ReadFile(keyPath)
	if err == nil && len(data) == 32 {
		var k masterKey
		copy(k[:], data)
		return k, nil
	}

	// 3. Generate new key and save.
	var k masterKey
	if _, err := io.ReadFull(rand.Reader, k[:]); err != nil {
		return masterKey{}, fmt.Errorf("generate secret key: %w", err)
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return masterKey{}, err
	}
	if err := os.WriteFile(keyPath, k[:], 0o600); err != nil {
		return masterKey{}, fmt.Errorf("save secret key: %w", err)
	}
	return k, nil
}

// encrypt encrypts plaintext with AES-256-GCM.
func encrypt(key masterKey, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// decrypt decrypts AES-256-GCM ciphertext.
func decrypt(key masterKey, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce, ct := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, ct, nil)
}
