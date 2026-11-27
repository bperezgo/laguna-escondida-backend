package service

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"

	"laguna-escondida/backend/internal/domain/dto"
)

// Shared by both Wompi webhooks: an organization's payment events and the platform's own billing events.

// validChecksum is Wompi's: SHA-256 of the named properties' values, the timestamp and the secret.
func validChecksum(event dto.WompiEvent, secret string) bool {
	if event.Signature.Checksum == "" || len(event.Signature.Properties) == 0 {
		return false
	}
	var b strings.Builder
	for _, property := range event.Signature.Properties {
		value, ok := propertyValue(event.Data, property)
		if !ok {
			return false
		}
		b.WriteString(value)
	}
	b.WriteString(strconv.FormatInt(event.Timestamp, 10))
	b.WriteString(secret)
	sum := sha256.Sum256([]byte(b.String()))
	want := strings.ToLower(event.Signature.Checksum)
	return subtle.ConstantTimeCompare([]byte(hex.EncodeToString(sum[:])), []byte(want)) == 1
}

// propertyValue reads a dotted path such as transaction.amount_in_cents from the event's data, as
// the text Wompi concatenates: a string unquoted, anything else as written.
func propertyValue(data map[string]json.RawMessage, path string) (string, bool) {
	parts := strings.Split(path, ".")
	current := data
	for i, part := range parts {
		raw, ok := current[part]
		if !ok {
			return "", false
		}
		if i == len(parts)-1 {
			var text string
			if err := json.Unmarshal(raw, &text); err == nil {
				return text, true
			}
			return strings.TrimSpace(string(raw)), true
		}
		current = nil
		if err := json.Unmarshal(raw, &current); err != nil {
			return "", false
		}
	}
	return "", false
}
