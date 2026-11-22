package apikeys

import (
	"net/http"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/domain/tenant"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (c *cloud) staffToken(orgID string, roleID int) string {
	c.t.Helper()
	userID := seedUser(c.t, orgID, roleID)
	token, err := service.NewJWTService("accept-secret", tenant.AnyOrganization()).GenerateToken(userID, orgID, "staff", []int{roleID})
	require.NoError(c.t, err)
	return token
}

func (c *cloud) issueOwnKey(adminToken, name string) dto.IssuedAPIKey {
	c.t.Helper()
	resp := c.withKey(adminToken, http.MethodPost, "/api/organization/api-keys", map[string]string{"name": name})
	require.Equal(c.t, http.StatusCreated, resp.status, string(resp.body))
	var issued dto.IssuedAPIKey
	resp.decode(c.t, &issued)
	return issued
}

func listedUserIDs(t *testing.T, c *cloud, adminToken string) []string {
	t.Helper()
	resp := c.withKey(adminToken, http.MethodGet, "/api/admin/users", nil)
	require.Equal(t, http.StatusOK, resp.status, string(resp.body))
	var list struct {
		Users []struct {
			ID string `json:"id"`
		} `json:"users"`
	}
	resp.decode(t, &list)
	ids := make([]string, 0, len(list.Users))
	for _, u := range list.Users {
		ids = append(ids, u.ID)
	}
	return ids
}

func TestOrgAPIKeys_AdminIssuesRotatesAndRevokes(t *testing.T) {
	c := newCloud(t)
	r := c.newRestaurant("Mojarra frita")
	admin := c.staffToken(r.orgID, permissions.RoleAdmin)

	old := c.issueOwnKey(admin, "console-local")
	assert.NotEqual(t, r.agentID, old.UserID, "the key gets its own user")
	var roles []int
	require.NoError(t, cloudDB.Raw("SELECT role_id FROM user_roles WHERE user_id = ?", old.UserID).Scan(&roles).Error)
	assert.Equal(t, []int{permissions.RoleAgent}, roles)
	assert.Equal(t, http.StatusCreated, orderStatus(c, r, old.Key), "the new key works")

	rotated := c.issueOwnKey(admin, "console-local-2")
	assert.NotEqual(t, old.UserID, rotated.UserID)
	assert.NotContains(t, listedUserIDs(t, c, admin), old.UserID, "a key's user is not in the users list")
	assert.Equal(t, http.StatusNotFound, c.withKey(admin, http.MethodPut, "/api/admin/users/"+old.UserID,
		map[string]any{"role_ids": []int{permissions.RoleAdmin}}).status, "nor editable")

	revoke := c.withKey(admin, http.MethodDelete, "/api/organization/api-keys/"+old.ID, nil)
	require.Equal(t, http.StatusNoContent, revoke.status, string(revoke.body))
	assert.Equal(t, http.StatusUnauthorized, c.withKey(old.Key, http.MethodGet, "/api/products", nil).status)
	assert.Equal(t, http.StatusCreated, orderStatus(c, r, rotated.Key), "the rotated key keeps working")
	var active bool
	require.NoError(t, cloudDB.Raw("SELECT active FROM users WHERE id = ?", old.UserID).Scan(&active).Error)
	assert.False(t, active, "revoking deactivates the key's user")

	list := c.withKey(admin, http.MethodGet, "/api/organization/api-keys", nil)
	require.Equal(t, http.StatusOK, list.status)
	var keys dto.APIKeyListResponse
	list.decode(t, &keys)
	byID := map[string]*dto.APIKey{}
	for _, k := range keys.APIKeys {
		byID[k.ID] = k
	}
	require.Contains(t, byID, old.ID)
	assert.NotNil(t, byID[old.ID].RevokedAt)
	assert.Contains(t, byID, r.keyID, "operator-issued keys are listed too")
}

func TestOrgAPIKeys_RevokingAnOperatorKeyLeavesItsUserActive(t *testing.T) {
	c := newCloud(t)
	r := c.newRestaurant("Bandeja paisa")
	admin := c.staffToken(r.orgID, permissions.RoleAdmin)

	require.Equal(t, http.StatusNoContent, c.withKey(admin, http.MethodDelete, "/api/organization/api-keys/"+r.keyID, nil).status)

	var active bool
	require.NoError(t, cloudDB.Raw("SELECT active FROM users WHERE id = ?", r.agentID).Scan(&active).Error)
	assert.True(t, active)
}

func TestOrgAPIKeys_OperatorRevokeDeactivatesAKeysUser(t *testing.T) {
	c := newCloud(t)
	r := c.newRestaurant("Sancocho")
	issued := c.issueOwnKey(c.staffToken(r.orgID, permissions.RoleAdmin), "console")

	require.Equal(t, http.StatusNoContent, c.admin(http.MethodDelete, "/api/platform/organizations/"+r.orgID+"/api-keys/"+issued.ID, nil).status)

	var active bool
	require.NoError(t, cloudDB.Raw("SELECT active FROM users WHERE id = ?", issued.UserID).Scan(&active).Error)
	assert.False(t, active)
}

func TestOrgAPIKeys_AnotherOrganizationsKeyIsNotFound(t *testing.T) {
	c := newCloud(t)
	a := c.newRestaurant("Arepa")
	b := c.newRestaurant("Ajiaco")

	resp := c.withKey(c.staffToken(a.orgID, permissions.RoleAdmin), http.MethodDelete, "/api/organization/api-keys/"+b.keyID, nil)

	assert.Equal(t, http.StatusNotFound, resp.status)
	assert.Equal(t, http.StatusCreated, orderStatus(c, b, b.key), "B's key still works")
}

func TestOrgAPIKeys_ManagerAndKeysAreRefused(t *testing.T) {
	c := newCloud(t)
	r := c.newRestaurant("Lechona")
	var before int64
	require.NoError(t, cloudDB.Raw("SELECT count(*) FROM users WHERE organization_id = ?", r.orgID).Scan(&before).Error)

	for name, bearer := range map[string]string{"manager": c.staffToken(r.orgID, permissions.RoleManager), "api key": r.key} {
		resp := c.withKey(bearer, http.MethodPost, "/api/organization/api-keys", map[string]string{"name": "k"})
		assert.Equal(t, http.StatusForbidden, resp.status, name)
	}

	var after int64
	require.NoError(t, cloudDB.Raw("SELECT count(*) FROM users WHERE organization_id = ?", r.orgID).Scan(&after).Error)
	assert.Equal(t, before+1, after, "only the manager seeded for the token was added")
}

func orderStatus(c *cloud, r *restaurant, key string) int {
	withKey := *r
	withKey.key = key
	return c.createTakeaway(&withKey).status
}
