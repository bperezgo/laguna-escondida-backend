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
	// ErrOrganizationInactive refuses an invitation or enrollment code of a suspended, pending or expired organization.
	ErrOrganizationInactive = errors.New("organization is not active")
	ErrPlanRequired         = errors.New("plan does not include this feature")
)

// PlanRequiredError carries the cheapest plan that includes the refused feature.
type PlanRequiredError struct {
	RequiredPlan string
}

func (e *PlanRequiredError) Error() string {
	return fmt.Sprintf("%s: requires plan %s", ErrPlanRequired.Error(), e.RequiredPlan)
}

func (e *PlanRequiredError) Is(target error) bool { return target == ErrPlanRequired }
