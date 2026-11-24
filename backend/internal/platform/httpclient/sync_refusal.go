package httpclient

import (
	"encoding/json"
	"fmt"
	"net/http"

	domainError "laguna-escondida/backend/internal/domain/error"
)

// syncStatusError is the error for a non-200 sync answer. It wraps ErrNodeRevoked or
// ErrNodeOrganizationSuspended when the cloud named that reason, so the box can tell the admin.
func syncStatusError(op string, status int, body []byte) error {
	var reason struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(body, &reason)
	switch {
	case status == http.StatusUnauthorized && reason.Code == "node_revoked":
		return fmt.Errorf("%s returned status %d: %s: %w", op, status, body, domainError.ErrNodeRevoked)
	case status == http.StatusForbidden && reason.Code == "organization_suspended":
		return fmt.Errorf("%s returned status %d: %s: %w", op, status, body, domainError.ErrNodeOrganizationSuspended)
	}
	return fmt.Errorf("%s returned status %d: %s", op, status, string(body))
}
