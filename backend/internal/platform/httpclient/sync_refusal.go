package httpclient

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	domainError "laguna-escondida/backend/internal/domain/error"
)

// syncStatusError is the error for a non-200 sync answer. A node refusal (invalid credentials,
// node_revoked, organization_suspended) wraps a NodeRefusalError carrying the cloud's Retry-After.
func syncStatusError(op string, resp *http.Response, body []byte) error {
	var answer struct {
		Error string `json:"error"`
		Code  string `json:"code"`
	}
	_ = json.Unmarshal(body, &answer)
	var reason error
	switch {
	case resp.StatusCode == http.StatusUnauthorized && answer.Code == "node_revoked":
		reason = domainError.ErrNodeRevoked
	case resp.StatusCode == http.StatusForbidden && answer.Code == "organization_suspended":
		reason = domainError.ErrNodeOrganizationSuspended
	case resp.StatusCode == http.StatusUnauthorized && answer.Error == "invalid node credentials":
		reason = domainError.ErrNodeKeyInvalid
	default:
		return fmt.Errorf("%s returned status %d: %s", op, resp.StatusCode, string(body))
	}
	refusal := &domainError.NodeRefusalError{Reason: reason, RetryAfter: retryAfter(resp.Header.Get("Retry-After"))}
	return fmt.Errorf("%s returned status %d: %s: %w", op, resp.StatusCode, body, refusal)
}

// retryAfter reads delay-seconds; an HTTP date or garbage is 0, leaving the default to the scheduler.
func retryAfter(header string) time.Duration {
	seconds, err := strconv.Atoi(header)
	if err != nil || seconds < 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}
