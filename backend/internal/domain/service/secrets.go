package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

func randomSecret(bytes int) (string, error) {
	buf := make([]byte, bytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate random secret: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func hashSecret(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

// readableAlphabet has no look-alikes (0/O, 1/I/L), for secrets a person reads and types.
const readableAlphabet = "23456789ABCDEFGHJKMNPQRSTUVWXYZ"

func randomReadable(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate random secret: %w", err)
	}
	out := make([]byte, n)
	for i, b := range buf {
		// 256 is not a multiple of 31; the bias (under 1%) doesn't matter for a short-lived secret.
		out[i] = readableAlphabet[int(b)%len(readableAlphabet)]
	}
	return string(out), nil
}
