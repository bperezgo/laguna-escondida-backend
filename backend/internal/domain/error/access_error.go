package error

import (
	"errors"
	"fmt"
)

var (
	// ErrOrganizationSuspended refuses a user or API key of a suspended organization.
	ErrOrganizationSuspended = errors.New("organization suspended")
	// ErrOrganizationPending refuses a signup the operator has not approved yet.
	ErrOrganizationPending = errors.New("approval pending")
	// ErrTrialExpired refuses an organization whose trial has ended, so the person contacts us.
	ErrTrialExpired = errors.New("trial ended")
	// ErrPaymentRequired refuses a paid organization whose period lapsed without a new payment.
	ErrPaymentRequired = errors.New("payment required")
	// ErrOrganizationInactive refuses an invitation or enrollment code of a suspended, pending, expired or unpaid organization.
	ErrOrganizationInactive = errors.New("organization is not active")
	ErrPlanRequired         = errors.New("plan does not include this feature")
)

// PendingApprovalError is ErrOrganizationPending for an admin, who continues signup with SetupToken.
type PendingApprovalError struct {
	SetupToken string
}

func (e *PendingApprovalError) Error() string { return ErrOrganizationPending.Error() }

func (e *PendingApprovalError) Is(target error) bool { return target == ErrOrganizationPending }

// StoppedError is ErrTrialExpired or ErrPaymentRequired for an admin, who pays their way back with SetupToken.
type StoppedError struct {
	Reason     error
	SetupToken string
}

func (e *StoppedError) Error() string { return e.Reason.Error() }

func (e *StoppedError) Is(target error) bool { return target == e.Reason }

// PlanRequiredError carries the cheapest plan that includes the refused feature.
type PlanRequiredError struct {
	RequiredPlan string
}

func (e *PlanRequiredError) Error() string {
	return fmt.Sprintf("%s: requires plan %s", ErrPlanRequired.Error(), e.RequiredPlan)
}

func (e *PlanRequiredError) Is(target error) bool { return target == ErrPlanRequired }
