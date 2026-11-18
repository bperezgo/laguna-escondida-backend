package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/tenant"
)

func newDomainEventMeta(ctx context.Context, name, subjectID string, at time.Time) (dto.DomainEventMeta, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return dto.DomainEventMeta{}, fmt.Errorf("generate event id: %w", err)
	}
	orgID, _ := tenant.OrgFrom(ctx)
	return dto.DomainEventMeta{ID: id.String(), Name: name, OrganizationID: orgID, Subject: subjectID, Time: at}, nil
}
