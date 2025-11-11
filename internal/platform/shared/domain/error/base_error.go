package error

import (
	"fmt"
	"runtime"
	"strings"
)

// ErrorCode is a type for error codes that can be extended
type ErrorCode string

// BaseError represents a structured error with code, message, and stack trace
type BaseError struct {
	code    ErrorCode
	message string
	field   interface{}
	stack   []string
	cause   error
}

// NewBaseError creates a new BaseError with the given code and message
func NewBaseError(code ErrorCode, message string) *BaseError {
	return &BaseError{
		code:    code,
		message: message,
		field:   nil,
		stack:   captureStack(),
	}
}

// NewBaseErrorWithField creates a new BaseError with the given code, message, and field value
func NewBaseErrorWithField(code ErrorCode, message string, fieldValue interface{}) *BaseError {
	return &BaseError{
		code:    code,
		message: message,
		field:   fieldValue,
		stack:   captureStack(),
	}
}

// Wrap wraps an existing error with a new BaseError
func Wrap(err error, code ErrorCode, message string) *BaseError {
	baseErr := &BaseError{
		code:    code,
		message: message,
		field:   nil,
		stack:   captureStack(),
		cause:   err,
	}

	// If the wrapped error is also a BaseError, merge stacks
	if wrappedBaseErr, ok := err.(*BaseError); ok {
		baseErr.stack = append(baseErr.stack, wrappedBaseErr.stack...)
	}

	return baseErr
}

// WrapWithField wraps an existing error with a new BaseError and field value context
func WrapWithField(err error, code ErrorCode, message string, fieldValue interface{}) *BaseError {
	baseErr := &BaseError{
		code:    code,
		message: message,
		field:   fieldValue,
		stack:   captureStack(),
		cause:   err,
	}

	// If the wrapped error is also a BaseError, merge stacks
	if wrappedBaseErr, ok := err.(*BaseError); ok {
		baseErr.stack = append(baseErr.stack, wrappedBaseErr.stack...)
	}

	return baseErr
}

// Error implements the error interface
func (e *BaseError) Error() string {
	fieldSuffix := ""
	if e.field != nil {
		fieldSuffix = fmt.Sprintf(" (value: %v)", e.field)
	}

	if e.cause != nil {
		return fmt.Sprintf("[%s] %s%s: %v", e.code, e.message, fieldSuffix, e.cause)
	}
	return fmt.Sprintf("[%s] %s%s", e.code, e.message, fieldSuffix)
}

// Unwrap returns the underlying error
func (e *BaseError) Unwrap() error {
	return e.cause
}

// GetCode returns the error code
func (e *BaseError) GetCode() ErrorCode {
	return e.code
}

// GetMessage returns the user-friendly message
func (e *BaseError) GetMessage() string {
	return e.message
}

// GetStack returns the stack trace as a slice of strings
func (e *BaseError) GetStack() []string {
	return e.stack
}

// GetFullStack returns the complete stack trace as a single string
func (e *BaseError) GetFullStack() string {
	return strings.Join(e.stack, "\n")
}

// GetFieldValue returns the field value that caused the error
func (e *BaseError) GetFieldValue() interface{} {
	return e.field
}

// captureStack captures the current stack trace
func captureStack() []string {
	const depth = 32
	var pcs [depth]uintptr
	n := runtime.Callers(3, pcs[:])
	frames := runtime.CallersFrames(pcs[:n])

	var stack []string
	for {
		frame, more := frames.Next()
		stack = append(stack, fmt.Sprintf("%s:%d %s", frame.File, frame.Line, frame.Function))
		if !more {
			break
		}
	}

	return stack
}
