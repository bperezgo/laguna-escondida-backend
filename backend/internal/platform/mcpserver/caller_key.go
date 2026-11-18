package mcpserver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"laguna-escondida/backend/internal/domain/dto"
)

// ErrCredentialRefused is a 401 from the backend for the caller's own key: there is no other
// credential to fall back to.
var ErrCredentialRefused = errors.New("credential refused: the backend did not accept this organization's API key")

var errNoCallerKey = errors.New("no organization API key on this request")

type callerKeyCtx struct{}

func withCallerKey(ctx context.Context, key string) context.Context {
	return context.WithValue(ctx, callerKeyCtx{}, key)
}

func callerKeyFrom(ctx context.Context) (string, bool) {
	key, ok := ctx.Value(callerKeyCtx{}).(string)
	return key, ok && key != ""
}

// RequireOrgAPIKey guards the customer endpoint: only `Bearer lgk_…` gets through, so a staff
// token never reaches the customer tools. The backend, not this server, decides if the key is valid.
func RequireOrgAPIKey(next http.Handler) http.Handler {
	return auth.RequireBearerToken(verifyOrgAPIKeyShape, nil)(next)
}

func verifyOrgAPIKeyShape(_ context.Context, token string, _ *http.Request) (*auth.TokenInfo, error) {
	if !strings.HasPrefix(token, dto.APIKeyPrefix) || len(token) == len(dto.APIKeyPrefix) {
		return nil, auth.ErrInvalidToken
	}
	sum := sha256.Sum256([]byte(token))
	// UserID binds an MCP session to the key that opened it, so one organization's session can't
	// be continued with another's key.
	return &auth.TokenInfo{UserID: hex.EncodeToString(sum[:]), Expiration: time.Now().Add(time.Hour)}, nil
}

// forwardCallerKey puts each request's own bearer on the context its tools run with.
func forwardCallerKey(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		if extra := req.GetExtra(); extra != nil && extra.Header != nil {
			if key, ok := strings.CutPrefix(extra.Header.Get("Authorization"), "Bearer "); ok && strings.HasPrefix(key, dto.APIKeyPrefix) {
				ctx = withCallerKey(ctx, key)
			}
		}
		return next(ctx, method, req)
	}
}
