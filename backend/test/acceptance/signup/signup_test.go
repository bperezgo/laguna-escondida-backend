package signup

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type me struct {
	Organization struct {
		ID          string     `json:"id"`
		Status      string     `json:"status"`
		Plan        string     `json:"plan"`
		TrialEndsAt *time.Time `json:"trial_ends_at"`
	} `json:"organization"`
}

func (c *cloud) signUp(email, restaurant, plan string) (token, orgID string) {
	c.t.Helper()
	resp := c.call(http.MethodPost, "/api/signup/google", nil, map[string]string{
		"id_token": googleToken(email, "Rosa Pérez"), "restaurant_name": restaurant, "plan": plan})
	require.Equal(c.t, http.StatusCreated, resp.status, string(resp.body))
	var signIn dto.SignInResponse
	resp.decode(c.t, &signIn)
	var m me
	c.as(signIn.Token, http.MethodGet, "/api/auth/me", nil).decode(c.t, &m)
	return signIn.Token, m.Organization.ID
}

func unique(prefix string) string { return prefix + "-" + uuid.NewString()[:8] + "@example.com" }

func TestSignup_TrialPlanAndExpiry(t *testing.T) {
	c := newCloud(t)
	email := unique("rosa")
	token, orgID := c.signUp(email, "Restaurante Doña Rosa", "starter")

	var m me
	c.as(token, http.MethodGet, "/api/auth/me", nil).decode(t, &m)
	assert.Equal(t, "trial", m.Organization.Status)
	assert.Equal(t, "starter", m.Organization.Plan)
	require.NotNil(t, m.Organization.TrialEndsAt)
	assert.WithinDuration(t, time.Now().Add(14*24*time.Hour), *m.Organization.TrialEndsAt, time.Minute)

	resp := c.as(token, http.MethodGet, "/api/invoices", nil)
	assert.Equal(t, http.StatusForbidden, resp.status)
	assert.Equal(t, "plan_required", resp.code(t))
	assert.Equal(t, http.StatusOK, c.as(token, http.MethodGet, "/api/products", nil).status, "core works on starter")

	again := c.call(http.MethodPost, "/api/signup/google", nil, map[string]string{
		"id_token": googleToken(email, "Rosa"), "restaurant_name": "Otro", "plan": "full"})
	assert.Equal(t, http.StatusOK, again.status, "a registered email is a sign-in, not a second account")

	past := time.Now().Add(-time.Minute).UTC().Format(time.RFC3339)
	require.Equal(t, http.StatusOK, c.operator(http.MethodPatch, "/api/platform/organizations/"+orgID, map[string]string{"trial_ends_at": past}).status)

	resp = c.as(token, http.MethodGet, "/api/products", nil)
	assert.Equal(t, http.StatusForbidden, resp.status)
	assert.Equal(t, "trial_expired", resp.code(t), "a token issued before expiry is refused after it")
	resp = c.call(http.MethodPost, "/api/auth/google", nil, map[string]string{"id_token": googleToken(email, "Rosa")})
	assert.Equal(t, http.StatusForbidden, resp.status)
	assert.Equal(t, "trial_expired", resp.code(t))

	require.Equal(t, http.StatusOK, c.operator(http.MethodPatch, "/api/platform/organizations/"+orgID,
		map[string]string{"status": "active", "plan": "einvoicing"}).status)
	assert.Equal(t, http.StatusOK, c.as(token, http.MethodGet, "/api/products", nil).status, "activated: access returns")
	assert.NotEqual(t, http.StatusForbidden, c.as(token, http.MethodGet, "/api/invoices", nil).status, "einvoicing now reaches invoices")
	assert.Equal(t, http.StatusOK, c.call(http.MethodPost, "/api/auth/google", nil, map[string]string{"id_token": googleToken(email, "Rosa")}).status)
}

func TestSignup_InviteAcceptAndSignIn(t *testing.T) {
	c := newCloud(t)
	token, _ := c.signUp(unique("owner"), "Casa Invita", "full")
	invitee := unique("contadora")

	resp := c.as(token, http.MethodPost, "/api/admin/invitations", map[string]any{"email": invitee, "name": "Contadora", "role_ids": []int{5}})
	require.Equal(t, http.StatusCreated, resp.status, string(resp.body))
	var created dto.InvitationWithLink
	resp.decode(t, &created)
	linkToken := created.Link[len("https://app.example.com/invitacion/"):]

	resp = c.call(http.MethodGet, "/api/invitations/"+linkToken, nil, nil)
	require.Equal(t, http.StatusOK, resp.status, string(resp.body))
	assert.Contains(t, string(resp.body), "Casa Invita")

	resp = c.call(http.MethodPost, "/api/invitations/"+linkToken+"/accept", nil, map[string]string{"password": "secreto123"})
	require.Equal(t, http.StatusCreated, resp.status, string(resp.body))

	resp = c.call(http.MethodPost, "/api/auth/signin", nil, map[string]string{"email": invitee, "password": "secreto123"})
	assert.Equal(t, http.StatusOK, resp.status, string(resp.body))
	resp = c.call(http.MethodPost, "/api/invitations/"+linkToken+"/accept", nil, map[string]string{"password": "secreto123"})
	assert.Equal(t, http.StatusBadRequest, resp.status, "a link is used once")
	assert.Equal(t, "invalid_or_expired_link", resp.code(t))
}

func TestSignup_EnrollWithACodeThenSync(t *testing.T) {
	c := newCloud(t)
	email := unique("caja")
	token, orgID := c.signUp(email, "Casa Caja", "starter")

	resp := c.as(token, http.MethodPost, "/api/edge/enrollment-codes", nil)
	require.Equal(t, http.StatusCreated, resp.status, string(resp.body))
	var issued dto.IssuedEnrollmentCode
	resp.decode(t, &issued)

	resp = c.call(http.MethodPost, "/api/edge/enroll-with-code", nil, map[string]any{"code": issued.Code, "name": "Caja principal"})
	require.Equal(t, http.StatusCreated, resp.status, string(resp.body))
	var box dto.EnrolledEdgeBox
	resp.decode(t, &box)
	assert.Equal(t, orgID, box.OrganizationID)

	resp = c.node(box.NodeID, box.NodeKey, http.MethodGet, "/api/sync/pull", nil)
	require.Equal(t, http.StatusOK, resp.status, string(resp.body))
	assert.Contains(t, string(resp.body), boxUsername(email), "the box user is pulled to the box")

	billID, invoiceID := uuid.NewString(), uuid.NewString()
	now := time.Now().UTC()
	billPayload, _ := json.Marshal(dto.BillSyncPayload{ID: billID, TotalAmount: decimal.NewFromInt(10000), PayAmount: decimal.NewFromInt(10000),
		PaymentMethod: "cash", CreatedAt: now, UpdatedAt: now})
	invoicePayload, _ := json.Marshal(dto.PendingInvoiceSyncPayload{ID: invoiceID, BillID: billID, PaymentCode: "10"})
	push := dto.SyncPushRequest{NodeID: box.NodeID, Ops: []dto.SyncOutboxEntry{
		{OpID: uuid.NewString(), OriginNodeID: box.NodeID, EntityType: dto.SyncEntityBill, EntityID: billID, Operation: dto.SyncOperationCreate, Payload: billPayload, Seq: 1, CreatedAt: now},
		{OpID: uuid.NewString(), OriginNodeID: box.NodeID, EntityType: dto.SyncEntityPendingInvoice, EntityID: invoiceID, Operation: dto.SyncOperationCreate, Payload: invoicePayload, Seq: 2, CreatedAt: now},
	}}
	resp = c.node(box.NodeID, box.NodeKey, http.MethodPost, "/api/sync/push", push)
	require.Equal(t, http.StatusOK, resp.status, string(resp.body))
	var bills, skipped int64
	require.NoError(t, cloudDB.Raw("SELECT count(*) FROM bills WHERE id = ?", billID).Scan(&bills).Error)
	require.NoError(t, cloudDB.Raw("SELECT count(*) FROM pending_invoices WHERE id = ? AND status = 'skipped'", invoiceID).Scan(&skipped).Error)
	assert.Equal(t, int64(1), bills, "a starter box's bill is stored")
	assert.Equal(t, int64(1), skipped, "its invoice is never sent to DIAN")

	past := time.Now().Add(-time.Minute).UTC().Format(time.RFC3339)
	require.Equal(t, http.StatusOK, c.operator(http.MethodPatch, "/api/platform/organizations/"+orgID, map[string]string{"trial_ends_at": past}).status)
	resp = c.node(box.NodeID, box.NodeKey, http.MethodGet, "/api/sync/pull", nil)
	assert.Equal(t, http.StatusForbidden, resp.status, string(resp.body))
	assert.Contains(t, string(resp.body), "organization_suspended", "an expired trial's box is refused as suspended")
}

func boxUsername(email string) string {
	local, _, _ := strings.Cut(email, "@")
	return local
}
