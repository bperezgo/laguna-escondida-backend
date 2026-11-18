package crypto

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"laguna-escondida/backend/internal/domain/ports"
)

func newTestCipher(t *testing.T) ports.SecretCipher {
	t.Helper()
	c, err := NewAESGCMCipher(bytes.Repeat([]byte{7}, 32))
	require.NoError(t, err)
	return c
}

func TestAESGCMCipher_RoundTrip(t *testing.T) {
	c := newTestCipher(t)
	enc, err := c.Encrypt([]byte("https://discord.com/api/webhooks/1/abc"))
	require.NoError(t, err)
	assert.Equal(t, byte(0x01), enc[0])
	assert.NotContains(t, string(enc), "discord")

	dec, err := c.Decrypt(enc)
	require.NoError(t, err)
	assert.Equal(t, "https://discord.com/api/webhooks/1/abc", string(dec))
}

func TestAESGCMCipher_EncryptUsesFreshNonce(t *testing.T) {
	c := newTestCipher(t)
	a, err := c.Encrypt([]byte("x"))
	require.NoError(t, err)
	b, err := c.Encrypt([]byte("x"))
	require.NoError(t, err)
	assert.NotEqual(t, a, b)
}

func TestAESGCMCipher_Decrypt_Tampered(t *testing.T) {
	c := newTestCipher(t)
	enc, err := c.Encrypt([]byte("secret"))
	require.NoError(t, err)
	enc[len(enc)-1] ^= 0xff

	_, err = c.Decrypt(enc)
	assert.ErrorIs(t, err, ErrInvalidCiphertext)
}

func TestAESGCMCipher_Decrypt_UnknownVersion(t *testing.T) {
	c := newTestCipher(t)
	enc, err := c.Encrypt([]byte("secret"))
	require.NoError(t, err)
	enc[0] = 0x02

	_, err = c.Decrypt(enc)
	assert.ErrorIs(t, err, ErrInvalidCiphertext)
}

func TestAESGCMCipher_Decrypt_TooShort(t *testing.T) {
	c := newTestCipher(t)
	_, err := c.Decrypt([]byte{0x01, 0x02})
	assert.ErrorIs(t, err, ErrInvalidCiphertext)
}

func TestAESGCMCipher_Decrypt_WrongKey(t *testing.T) {
	c := newTestCipher(t)
	enc, err := c.Encrypt([]byte("secret"))
	require.NoError(t, err)
	other, err := NewAESGCMCipher(bytes.Repeat([]byte{9}, 32))
	require.NoError(t, err)

	_, err = other.Decrypt(enc)
	assert.ErrorIs(t, err, ErrInvalidCiphertext)
}

func TestNewAESGCMCipher_BadKeyLength(t *testing.T) {
	for _, n := range []int{0, 16, 31, 33} {
		_, err := NewAESGCMCipher(make([]byte, n))
		assert.Error(t, err, "length %d", n)
	}
}
