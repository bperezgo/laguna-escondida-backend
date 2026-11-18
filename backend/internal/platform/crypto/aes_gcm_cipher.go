package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"

	"laguna-escondida/backend/internal/domain/ports"
)

const formatVersion byte = 0x01

const keySize = 32

var ErrInvalidCiphertext = errors.New("invalid ciphertext")

type aesGCMCipher struct {
	aead cipher.AEAD
}

// NewAESGCMCipher stores `0x01|nonce|ciphertext`; the version byte leaves room to change the scheme later.
func NewAESGCMCipher(key []byte) (ports.SecretCipher, error) {
	if len(key) != keySize {
		return nil, fmt.Errorf("key must be %d bytes, got %d", keySize, len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("creating cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("creating gcm: %w", err)
	}
	return &aesGCMCipher{aead: aead}, nil
}

func (c *aesGCMCipher) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generating nonce: %w", err)
	}
	out := make([]byte, 0, 1+len(nonce)+len(plaintext)+c.aead.Overhead())
	out = append(out, formatVersion)
	out = append(out, nonce...)
	return c.aead.Seal(out, nonce, plaintext, nil), nil
}

func (c *aesGCMCipher) Decrypt(ciphertext []byte) ([]byte, error) {
	nonceSize := c.aead.NonceSize()
	if len(ciphertext) < 1+nonceSize+c.aead.Overhead() {
		return nil, ErrInvalidCiphertext
	}
	if ciphertext[0] != formatVersion {
		return nil, fmt.Errorf("%w: unknown version %#x", ErrInvalidCiphertext, ciphertext[0])
	}
	nonce := ciphertext[1 : 1+nonceSize]
	plaintext, err := c.aead.Open(nil, nonce, ciphertext[1+nonceSize:], nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidCiphertext, err)
	}
	return plaintext, nil
}
