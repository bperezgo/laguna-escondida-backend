package apikeys

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/domain/tenant"
	"laguna-escondida/backend/internal/platform/postgres/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func productNames(t *testing.T, resp response) []string {
	t.Helper()
	require.Equal(t, http.StatusOK, resp.status, string(resp.body))
	var list struct {
		Products []struct {
			Name string `json:"name"`
		} `json:"products"`
	}
	resp.decode(t, &list)
	names := make([]string, 0, len(list.Products))
	for _, p := range list.Products {
		names = append(names, p.Name)
	}
	return names
}

// 5.1: one cloud, two restaurants, each with its own agent key.
func TestAPIKeys_EachKeyActsOnlyForItsOrganization(t *testing.T) {
	c := newCloud(t)
	a := c.newRestaurant("Mojarra frita")
	b := c.newRestaurant("Bandeja paisa")

	assert.Equal(t, []string{"Mojarra frita"}, productNames(t, c.withKey(a.key, http.MethodGet, "/api/products", nil)))
	assert.Equal(t, []string{"Bandeja paisa"}, productNames(t, c.withKey(b.key, http.MethodGet, "/api/products", nil)))

	orderOfB := orderID(t, c.createTakeaway(b))
	orderOfA := orderID(t, c.createTakeaway(a))

	var createdIn struct {
		OrganizationID string
		CreatedBy      string
	}
	require.NoError(t, cloudDB.Raw("SELECT organization_id, created_by FROM open_bills WHERE id = ?", orderOfA).Scan(&createdIn).Error)
	assert.Equal(t, a.orgID, createdIn.OrganizationID, "the order lands in the key's organization")
	assert.Equal(t, a.agentID, createdIn.CreatedBy, "created_by is the key's user")

	list := c.withKey(a.key, http.MethodGet, "/api/orders", nil)
	require.Equal(t, http.StatusOK, list.status)
	assert.Contains(t, string(list.body), orderOfA)
	assert.NotContains(t, string(list.body), orderOfB, "B's orders are not listed for A's key")

	assert.Equal(t, http.StatusNotFound, c.withKey(a.key, http.MethodGet, "/api/orders/"+orderOfB, nil).status)
	update := c.withKey(a.key, http.MethodPut, "/api/orders/"+orderOfB, map[string]any{
		"products": []map[string]any{{"open_bill_product_id": uuid.NewString(), "product_id": a.productID, "quantity": 3}},
	})
	assert.Equal(t, http.StatusNotFound, update.status, string(update.body))
	assert.Equal(t, http.StatusNotFound, c.withKey(a.key, http.MethodDelete, "/api/orders/"+orderOfB, nil).status)

	var stillThere int64
	require.NoError(t, cloudDB.Raw("SELECT count(*) FROM open_bills WHERE id = ? AND organization_id = ? AND deleted_at IS NULL",
		orderOfB, b.orgID).Scan(&stillThere).Error)
	assert.Equal(t, int64(1), stillThere, "B's order is untouched")
}

func TestAPIKeys_TheKeysLimitsAreTheUsersLimits(t *testing.T) {
	c := newCloud(t)
	r := c.newRestaurant("Mojarra frita")
	order := orderID(t, c.createTakeaway(r))

	pay := c.withKey(r.key, http.MethodPost, "/api/orders/pay-order", map[string]any{"order_id": order})
	assert.Equal(t, http.StatusForbidden, pay.status, string(pay.body))
	assert.Equal(t, http.StatusForbidden, c.withKey(r.key, http.MethodPost, "/api/products", map[string]any{}).status)

	var paid int64
	require.NoError(t, cloudDB.Raw("SELECT count(*) FROM open_bills WHERE id = ? AND paid_at IS NOT NULL", order).Scan(&paid).Error)
	assert.Zero(t, paid, "nothing was paid")
}

// 5.2: each way a key stops working gives the same 401 as a key that never existed.
func TestAPIKeys_StopWorkingWithTheirUserOrOrganization(t *testing.T) {
	c := newCloud(t)
	r := c.newRestaurant("Mojarra frita")
	probe := func() response { return c.withKey(r.key, http.MethodGet, "/api/products", nil) }
	require.Equal(t, http.StatusOK, probe().status)

	unknown := c.withKey("lgk_"+uuid.NewString(), http.MethodGet, "/api/products", nil)
	require.Equal(t, http.StatusUnauthorized, unknown.status)

	require.NoError(t, cloudDB.Exec("UPDATE users SET active = false WHERE id = ?", r.agentID).Error)
	deactivated := probe()
	require.NoError(t, cloudDB.Exec("UPDATE users SET active = true WHERE id = ?", r.agentID).Error)
	require.Equal(t, http.StatusOK, probe().status, "works again once the user is active")

	suspend := c.admin(http.MethodPatch, "/api/platform/organizations/"+r.orgID, map[string]string{"status": "suspended"})
	require.Equal(t, http.StatusOK, suspend.status, string(suspend.body))
	suspended := probe()
	require.Equal(t, http.StatusOK, c.admin(http.MethodPatch, "/api/platform/organizations/"+r.orgID, map[string]string{"status": "active"}).status)
	require.Equal(t, http.StatusOK, probe().status, "works again once the organization is active")

	require.Equal(t, http.StatusNoContent, c.admin(http.MethodDelete, "/api/platform/organizations/"+r.orgID+"/api-keys/"+r.keyID, nil).status)
	revoked := probe()

	for name, resp := range map[string]response{"deactivated user": deactivated, "suspended organization": suspended, "revoked key": revoked} {
		assert.Equal(t, http.StatusUnauthorized, resp.status, name)
		assert.JSONEq(t, string(unknown.body), string(resp.body), "%s reads like an unknown key", name)
	}

	require.Equal(t, http.StatusOK, c.admin(http.MethodPatch, "/api/platform/organizations/"+r.orgID, map[string]string{"status": "active"}).status)
	assert.Equal(t, http.StatusUnauthorized, probe().status, "revocation is permanent")

	var list dto.APIKeyListResponse
	listed := c.admin(http.MethodGet, "/api/platform/organizations/"+r.orgID+"/api-keys", nil)
	require.Equal(t, http.StatusOK, listed.status)
	listed.decode(t, &list)
	require.Len(t, list.APIKeys, 1)
	assert.NotNil(t, list.APIKeys[0].RevokedAt, "listed with its revocation time")
	assert.NotContains(t, string(listed.body), r.key)
	require.Eventually(t, func() bool {
		var lastUsed *time.Time
		_ = cloudDB.Raw("SELECT last_used_at FROM api_keys WHERE id = ?", r.keyID).Scan(&lastUsed).Error
		return lastUsed != nil
	}, 5*time.Second, 50*time.Millisecond, "a used key records when")
}

func TestAPIKeys_RevokingThroughAnotherOrganizationIsNotFound(t *testing.T) {
	c := newCloud(t)
	a := c.newRestaurant("Mojarra frita")
	b := c.newRestaurant("Bandeja paisa")

	resp := c.admin(http.MethodDelete, "/api/platform/organizations/"+a.orgID+"/api-keys/"+b.keyID, nil)

	assert.Equal(t, http.StatusNotFound, resp.status)
	assert.Equal(t, http.StatusOK, c.withKey(b.key, http.MethodGet, "/api/products", nil).status, "B's key still works")
}

func TestAPIKeys_IssuingIsLimitedToAgentOnlyUsers(t *testing.T) {
	c := newCloud(t)
	a := c.newRestaurant("Mojarra frita")
	b := c.newRestaurant("Bandeja paisa")
	manager := seedUser(t, a.orgID, permissions.RoleManager)

	for name, userID := range map[string]string{"manager": manager, "agent of another organization": b.agentID} {
		resp := c.admin(http.MethodPost, "/api/platform/organizations/"+a.orgID+"/api-keys", map[string]string{"user_id": userID, "name": "k"})
		assert.Equal(t, http.StatusBadRequest, resp.status, name)
	}
	var count int64
	require.NoError(t, cloudDB.Raw("SELECT count(*) FROM api_keys WHERE organization_id = ?", a.orgID).Scan(&count).Error)
	assert.Equal(t, int64(1), count, "only the agent's own key exists")

	var hash string
	require.NoError(t, cloudDB.Raw("SELECT hash FROM api_keys WHERE id = ?", a.keyID).Scan(&hash).Error)
	assert.NotContains(t, hash, a.key[4:], "the stored record holds no plaintext")
}

// 2.2 and 5.3: the lookup finds a key without the org scope, and must keep doing so under RLS.
func TestAPIKeyRepository_FindByHash_RunsWithoutTheOrgScope(t *testing.T) {
	c := newCloud(t)
	a := c.newRestaurant("Mojarra frita")
	b := c.newRestaurant("Bandeja paisa")
	keys := service.NewAPIKeyService(repository.NewAPIKeyRepository(cloudDB), repository.NewOrganizationRepository(cloudDB),
		repository.NewUserRepository(cloudDB), repository.NewUserRoleRepository(cloudDB), slog.New(slog.NewTextHandler(io.Discard, nil)))

	principal, err := keys.Authenticate(context.Background(), a.key)
	require.NoError(t, err, "found from a context with no organization")
	assert.Equal(t, &dto.APIKeyPrincipal{
		KeyID: a.keyID, UserID: a.agentID, Username: principal.Username, OrganizationID: a.orgID, RoleIDs: []int{permissions.RoleAgent},
	}, principal)

	// Any filter on the context's organization would hide A's key from a context carrying B.
	_, err = keys.Authenticate(tenant.WithOrg(context.Background(), b.orgID), a.key)
	assert.NoError(t, err, "the lookup must not be scoped to the context's organization")

	require.NoError(t, keys.Revoke(context.Background(), a.orgID, a.keyID))
	principal, err = keys.Authenticate(context.Background(), a.key)
	assert.Nil(t, principal, "nothing comes back for a revoked key")
	assert.ErrorIs(t, err, domainError.ErrCredentialRefused)
}
