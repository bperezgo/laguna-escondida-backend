package service

import (
	"context"
	"errors"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNodeService_OrganizationOf_RegisteredNode(t *testing.T) {
	repo := mocks.NewMockNodeRepository(t)
	repo.On("FindByID", context.Background(), "node-1").Return(&dto.Node{ID: "node-1", OrganizationID: testOrgA}, nil)

	orgID, err := NewNodeService(repo).OrganizationOf(context.Background(), "node-1")

	require.NoError(t, err)
	assert.Equal(t, testOrgA, orgID)
}

func TestNodeService_OrganizationOf_Errors(t *testing.T) {
	cases := []struct {
		name    string
		repoErr error
	}{
		{"unregistered node", domainError.ErrNodeNotFound},
		{"repository failure", errors.New("connection refused")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := mocks.NewMockNodeRepository(t)
			repo.On("FindByID", context.Background(), "node-x").Return(nil, tc.repoErr)

			orgID, err := NewNodeService(repo).OrganizationOf(context.Background(), "node-x")

			assert.Empty(t, orgID)
			assert.ErrorIs(t, err, tc.repoErr)
		})
	}
}
