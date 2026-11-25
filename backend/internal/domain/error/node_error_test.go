package error

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNodeRefusalError_UnwrapsToReason(t *testing.T) {
	for _, reason := range []error{ErrNodeKeyInvalid, ErrNodeRevoked, ErrNodeOrganizationSuspended} {
		t.Run(reason.Error(), func(t *testing.T) {
			err := fmt.Errorf("pull: %w", &NodeRefusalError{Reason: reason, RetryAfter: 6 * time.Hour})

			assert.ErrorIs(t, err, reason)
			var refusal *NodeRefusalError
			require.ErrorAs(t, err, &refusal)
			assert.Equal(t, 6*time.Hour, refusal.RetryAfter)
			assert.Equal(t, reason.Error(), refusal.Error())
		})
	}
}

func TestNodeRefusalError_DoesNotMatchOtherReasons(t *testing.T) {
	err := &NodeRefusalError{Reason: ErrNodeRevoked}

	assert.False(t, errors.Is(err, ErrNodeOrganizationSuspended))
}
