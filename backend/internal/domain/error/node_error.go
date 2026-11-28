package error

import (
	"errors"
	"time"
)

var (
	// ErrNodeNotFound means no node is registered with the id.
	ErrNodeNotFound = errors.New("node not registered")
	// ErrNodeKeyInvalid covers a missing key, a wrong key and an unknown node alike, so a caller can't tell them apart.
	ErrNodeKeyInvalid = errors.New("invalid node key")
	// ErrNodeRevoked is only returned to a caller that presented the node's correct key.
	ErrNodeRevoked = errors.New("node revoked")
	// ErrNodeOrganizationSuspended is only returned to a caller that presented the node's correct key.
	ErrNodeOrganizationSuspended = errors.New("node organization suspended")
	ErrNodeNameRequired          = errors.New("box name is required")
	// ErrPullApplyFailed means the cloud answered a pull but its data could not be applied on the box.
	ErrPullApplyFailed = errors.New("pull data could not be applied")
)

// ErrBoxAlreadyEnrolled is also what a lost enrollment race reports.
var ErrBoxAlreadyEnrolled = errors.New("organization already has an active box")

// BoxAlreadyEnrolledError carries the active box so the caller can decide whether to replace it.
type BoxAlreadyEnrolledError struct {
	Name       string
	EnrolledAt *time.Time
	LastSeenAt *time.Time
}

func (e *BoxAlreadyEnrolledError) Error() string { return ErrBoxAlreadyEnrolled.Error() }

func (e *BoxAlreadyEnrolledError) Is(target error) bool { return target == ErrBoxAlreadyEnrolled }

// NodeRefusalError is a cloud refusal of this node, with the delay the cloud asked for (0 when it gave none).
type NodeRefusalError struct {
	Reason     error
	RetryAfter time.Duration
}

func (e *NodeRefusalError) Error() string { return e.Reason.Error() }

func (e *NodeRefusalError) Unwrap() error { return e.Reason }
