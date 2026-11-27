package edgeboot

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"laguna-escondida/backend/internal/domain/tenant"
	"laguna-escondida/backend/internal/platform/postgres/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getContext(t *testing.T, baseURL string) map[string]any {
	t.Helper()
	resp, err := http.Get(baseURL + "/api/auth/context")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, string(raw))
	var body map[string]any
	require.NoError(t, json.Unmarshal(raw, &body))
	return body
}

// A fresh box answers that nobody can sign in yet, and says so without any user data; the first
// pulled user flips it, and it stays flipped.
func TestBox_SignInContext_WaitsForTheFirstUser(t *testing.T) {
	skipUnlessEnabled(t)
	box := boxes[otherBoxDBName]
	require.NoError(t, box.owner.Exec("UPDATE users SET active = false WHERE organization_id = ?", boxOrgID).Error)
	t.Cleanup(func() { box.owner.Exec("UPDATE users SET active = true WHERE organization_id = ?", boxOrgID) })
	server, _ := startBox(t, box, "secret-"+uuid.NewString())

	body := getContext(t, server.URL)
	assert.Equal(t, "edge", body["mode"])
	assert.Equal(t, false, body["users_ready"])
	assert.Equal(t, "waiting", body["sync"])
	assert.ElementsMatch(t, []string{"mode", "organization", "users_ready", "sync"}, keysOf(body))

	require.NoError(t, box.owner.Exec("UPDATE users SET active = true WHERE organization_id = ?", boxOrgID).Error)
	body = getContext(t, server.URL)
	assert.Equal(t, true, body["users_ready"])
	assert.Nil(t, body["sync"])
}

func keysOf(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func TestUserRepository_AnyActive_CountsOnlyTheCallersActiveUsers(t *testing.T) {
	skipUnlessEnabled(t)
	box := boxes[otherBoxDBName]
	orgs := repository.NewBoxOrganizationRepository(box.app)
	users := repository.NewUserRepository(box.app)
	emptyOrg := uuid.NewString()
	t.Cleanup(func() { box.owner.Exec("DELETE FROM organizations WHERE id = ?", emptyOrg) })
	require.NoError(t, orgs.Ensure(t.Context(), emptyOrg))

	has, err := users.AnyActive(tenant.WithOrg(t.Context(), emptyOrg))
	require.NoError(t, err)
	assert.False(t, has, "another organization's users don't count")

	has, err = users.AnyActive(tenant.WithOrg(t.Context(), boxOrgID))
	require.NoError(t, err)
	assert.True(t, has)

	_, err = users.AnyActive(t.Context())
	assert.Error(t, err, "no organization in the context is refused")
}
