package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/domain/tenant"

	"github.com/google/uuid"
)

const (
	EnrollmentCodeLifetime = 30 * time.Minute
	enrollmentCodeLength   = 8
)

// EnrollmentCodeService lets the installer enroll a box with a short-lived code instead of the admin's password.
type EnrollmentCodeService struct {
	codes   ports.EnrollmentCodeRepository
	nodes   *NodeService
	access  *OrganizationAccessService
	uow     ports.UnitOfWork
	now     func() time.Time
	newCode func() (string, error)
}

func NewEnrollmentCodeService(codes ports.EnrollmentCodeRepository, nodes *NodeService, access *OrganizationAccessService, uow ports.UnitOfWork) *EnrollmentCodeService {
	return &EnrollmentCodeService{codes: codes, nodes: nodes, access: access, uow: uow, now: time.Now,
		newCode: func() (string, error) { return randomReadable(enrollmentCodeLength) }}
}

// Issue invalidates the organization's earlier unused codes, so only the newest works.
func (s *EnrollmentCodeService) Issue(ctx context.Context, createdBy string) (*dto.IssuedEnrollmentCode, error) {
	orgID, err := tenant.RequireOrg(ctx)
	if err != nil {
		return nil, err
	}
	if err = s.access.Usable(ctx, orgID); err != nil {
		return nil, err
	}
	code, err := s.newCode()
	if err != nil {
		return nil, err
	}
	now := s.now().UTC()
	record := &dto.EnrollmentCode{ID: uuid.Must(uuid.NewV7()).String(), CreatedBy: createdBy, ExpiresAt: now.Add(EnrollmentCodeLifetime), CreatedAt: now}
	err = s.uow.Do(ctx, func(ctx context.Context) error {
		if invalidateErr := s.codes.InvalidateUnused(ctx, now); invalidateErr != nil {
			return invalidateErr
		}
		return s.codes.Create(ctx, record, hashSecret(NormalizeEnrollmentCode(code)))
	})
	if err != nil {
		return nil, fmt.Errorf("issue enrollment code: %w", err)
	}
	return &dto.IssuedEnrollmentCode{Code: code[:4] + "-" + code[4:], ExpiresAt: record.ExpiresAt}, nil
}

// EnrollWithCode is POST /api/edge/enroll by the code's organization; only a 201 uses the code up,
// so a 409 leaves it for the retry with replace.
func (s *EnrollmentCodeService) EnrollWithCode(ctx context.Context, req *dto.EnrollWithCodeRequest) (*dto.EnrolledEdgeBox, error) {
	normalized := NormalizeEnrollmentCode(req.Code)
	if len(normalized) != enrollmentCodeLength {
		return nil, domainError.ErrEnrollmentCodeInvalid
	}
	var enrolled *dto.EnrolledEdgeBox
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		code, err := s.codes.FindByHashForUpdate(ctx, hashSecret(normalized))
		if err != nil {
			return err
		}
		now := s.now().UTC()
		if code.UsedAt != nil || !now.Before(code.ExpiresAt) {
			return domainError.ErrEnrollmentCodeInvalid
		}
		if err = s.access.Usable(ctx, code.OrganizationID); err != nil {
			return err
		}
		enrolled, err = s.nodes.Enroll(ctx, code.OrganizationID, &dto.EnrollEdgeBoxRequest{Name: req.Name, Replace: req.Replace})
		if err != nil {
			return err
		}
		return s.codes.MarkUsed(ctx, code.ID, now)
	})
	if err != nil {
		return nil, err
	}
	return enrolled, nil
}

// NormalizeEnrollmentCode ignores case, spaces and the hyphen, so "k7f2 9qxm" is K7F2-9QXM.
func NormalizeEnrollmentCode(code string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(code) {
		if r == ' ' || r == '-' || r == '\t' {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
