package edgeboot

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"testing"

	"laguna-escondida/backend/internal/platform/postgres/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type logBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *logBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

func (l *logBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

func skipUnlessEnabled(t *testing.T) {
	t.Helper()
	if !gateEnabled() {
		t.Skip("Skipping acceptance test. Set RUN_ACCEPTANCE_TESTS=true to run (needs Docker or DB_HOST).")
	}
}

func TestBoxSecretRepository_GetOrCreate(t *testing.T) {
	skipUnlessEnabled(t)
	repo := repository.NewBoxSecretRepository(boxes[firstBoxDBName].app)
	name := "probe-" + uuid.NewString()

	first, err := repo.GetOrCreate(t.Context(), name, "first")
	require.NoError(t, err)
	second, err := repo.GetOrCreate(t.Context(), name, "second")
	require.NoError(t, err)

	assert.Equal(t, "first", first)
	assert.Equal(t, "first", second, "a stored secret is never replaced")
}

func TestBoxSecretRepository_ConcurrentFirstCallsAgree(t *testing.T) {
	skipUnlessEnabled(t)
	repo := repository.NewBoxSecretRepository(boxes[firstBoxDBName].app)
	name := "race-" + uuid.NewString()

	const callers = 8
	results := make([]string, callers)
	var wg sync.WaitGroup
	for i := range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			value, err := repo.GetOrCreate(t.Context(), name, uuid.NewString())
			assert.NoError(t, err)
			results[i] = value
		}()
	}
	wg.Wait()

	for _, value := range results {
		assert.Equal(t, results[0], value)
	}
	assert.NotEmpty(t, results[0])
}

type apiResponse struct {
	status int
	body   []byte
}

func call(t *testing.T, base, method, path, token string, body any) apiResponse {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, base+path, reader)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return apiResponse{status: resp.StatusCode, body: raw}
}

func signIn(t *testing.T, base string) string {
	t.Helper()
	resp := call(t, base, http.MethodPost, "/api/auth/signin", "", map[string]string{"username": boxUsername, "password": boxPassword})
	require.Equal(t, http.StatusOK, resp.status, string(resp.body))
	var out struct {
		Token string `json:"token"`
	}
	require.NoError(t, json.Unmarshal(resp.body, &out))
	require.NotEmpty(t, out.Token)
	return out.Token
}

// The box has no JWT_SECRET, ADMIN_API_KEY or STORAGE_*: it signs with a secret of its own.
func TestEdgeBoot_WithNoCloudSecretsKeepsItsOwnSigningSecret(t *testing.T) {
	skipUnlessEnabled(t)
	first, second := boxes[firstBoxDBName], boxes[otherBoxDBName]
	for _, db := range []boxDB{first, second} {
		require.NoError(t, db.owner.Exec("DELETE FROM box_secrets").Error)
	}

	box, logs := startBox(t, first, "")
	token := signIn(t, box.URL)
	assert.Equal(t, http.StatusOK, call(t, box.URL, http.MethodGet, "/api/auth/me", token, nil).status)

	var stored string
	require.NoError(t, first.owner.Raw("SELECT value FROM box_secrets WHERE name = 'jwt_signing'").Scan(&stored).Error)
	assert.NotEmpty(t, stored)
	assert.NotContains(t, logs.String(), stored, "the secret is never logged")

	t.Run("a restart keeps the session", func(t *testing.T) {
		restarted, _ := startBox(t, first, "")
		assert.Equal(t, http.StatusOK, call(t, restarted.URL, http.MethodGet, "/api/auth/me", token, nil).status)
	})

	t.Run("another box rejects the token", func(t *testing.T) {
		other, _ := startBox(t, second, "")
		assert.Equal(t, http.StatusUnauthorized, call(t, other.URL, http.MethodGet, "/api/auth/me", token, nil).status)
	})

	t.Run("the environment value wins over the stored one", func(t *testing.T) {
		withEnv, _ := startBox(t, first, "from-the-environment")
		assert.Equal(t, http.StatusUnauthorized, call(t, withEnv.URL, http.MethodGet, "/api/auth/me", token, nil).status)
		envToken := signIn(t, withEnv.URL)
		assert.Equal(t, http.StatusOK, call(t, withEnv.URL, http.MethodGet, "/api/auth/me", envToken, nil).status)

		var after string
		require.NoError(t, first.owner.Raw("SELECT value FROM box_secrets WHERE name = 'jwt_signing'").Scan(&after).Error)
		assert.Equal(t, stored, after, "the stored secret is untouched")
	})
}
