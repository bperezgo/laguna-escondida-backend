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

// signUp creates a pending account, has the operator start its trial, and signs its admin in.
func (c *cloud) signUp(email, restaurant, plan string) (token, orgID string) {
	c.t.Helper()
	orgID = c.signUpPending(email, restaurant, plan)
	require.Equal(c.t, http.StatusOK, c.operator(http.MethodPost, "/api/platform/organizations/"+orgID+"/start-trial", nil).status)
	resp := c.call(http.MethodPost, "/api/auth/google", nil, map[string]string{"id_token": googleToken(email, "Rosa Pérez")})
	require.Equal(c.t, http.StatusOK, resp.status, string(resp.body))
	var signIn dto.SignInResponse
	resp.decode(c.t, &signIn)
	return signIn.Token, orgID
}

func (c *cloud) signUpPending(email, restaurant, plan string) (orgID string) {
	c.t.Helper()
	orgID, _ = c.signUpForSetup(email, restaurant, plan)
	return orgID
}

// signUpForSetup creates a pending account and returns the setup token its 201 carries.
func (c *cloud) signUpForSetup(email, restaurant, plan string) (orgID, setupToken string) {
	c.t.Helper()
	resp := c.call(http.MethodPost, "/api/signup/google", nil, map[string]string{
		"id_token": googleToken(email, "Rosa Pérez"), "restaurant_name": restaurant, "plan": plan})
	require.Equal(c.t, http.StatusCreated, resp.status, string(resp.body))
	var created map[string]string
	resp.decode(c.t, &created)
	setupToken = created["setup_token"]
	require.NotEmpty(c.t, setupToken)
	delete(created, "setup_token")
	assert.Equal(c.t, map[string]string{"status": "pending", "organization_name": restaurant, "plan": plan}, created, "no sign-in token")
	require.NoError(c.t, cloudDB.Raw("SELECT organization_id FROM users WHERE email = ?", email).Scan(&orgID).Error)
	require.NotEmpty(c.t, orgID)
	return orgID, setupToken
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

func TestSignup_PendingUntilTheTrialStarts(t *testing.T) {
	c := newCloud(t)
	email := unique("pendiente")
	orgID := c.signUpPending(email, "Casa Pendiente", "einvoicing")

	var stored struct {
		Status      string
		TrialEndsAt *time.Time
	}
	require.NoError(t, cloudDB.Raw("SELECT status, trial_ends_at FROM organizations WHERE id = ?", orgID).Scan(&stored).Error)
	assert.Equal(t, "pending", stored.Status)
	assert.Nil(t, stored.TrialEndsAt)

	resp := c.call(http.MethodPost, "/api/auth/google", nil, map[string]string{"id_token": googleToken(email, "Rosa")})
	assert.Equal(t, http.StatusForbidden, resp.status)
	assert.Equal(t, "pending_approval", resp.code(t))
	resp = c.call(http.MethodPost, "/api/signup/google", nil, map[string]string{
		"id_token": googleToken(email, "Rosa"), "restaurant_name": "Otra", "plan": "full"})
	assert.Equal(t, http.StatusForbidden, resp.status, "signing up again while pending creates nothing")
	assert.Equal(t, "pending_approval", resp.code(t))

	resp = c.operator(http.MethodGet, "/api/platform/organizations?status=pending", nil)
	require.Equal(t, http.StatusOK, resp.status)
	var list struct{ Organizations []dto.Organization }
	resp.decode(t, &list)
	ids := map[string]string{}
	for _, o := range list.Organizations {
		assert.Equal(t, "pending", o.Status)
		ids[o.ID] = o.Plan
	}
	assert.Equal(t, "einvoicing", ids[orgID], "the waiting signup is listed with its plan")

	resp = c.operator(http.MethodPatch, "/api/platform/organizations/"+orgID, map[string]string{"status": "trial", "trial_ends_at": "2026-12-31T23:59:59-05:00"})
	assert.Equal(t, http.StatusBadRequest, resp.status, "a trial is started only by start-trial")

	resp = c.operator(http.MethodPost, "/api/platform/organizations/"+orgID+"/start-trial", nil)
	require.Equal(t, http.StatusOK, resp.status, string(resp.body))
	var started dto.Organization
	resp.decode(t, &started)
	assert.Equal(t, "trial", started.Status)
	assert.Equal(t, "einvoicing", started.Plan)
	require.NotNil(t, started.TrialEndsAt)
	assert.WithinDuration(t, time.Now().Add(14*24*time.Hour), *started.TrialEndsAt, time.Minute)

	resp = c.operator(http.MethodPost, "/api/platform/organizations/"+orgID+"/start-trial", nil)
	assert.Equal(t, http.StatusConflict, resp.status)
	assert.Equal(t, "not_pending", resp.code(t))

	assert.Equal(t, http.StatusOK, c.call(http.MethodPost, "/api/auth/google", nil, map[string]string{"id_token": googleToken(email, "Rosa")}).status,
		"approved: the admin signs in at once")
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
