// Package errors provides centralized BPN-style application errors with stable codes.
package errors

import (
	"errors"
	"fmt"
)

// AppError is the standard error type for handlers and services.
type AppError struct {
	HTTPCode int
	Code     string
	Message  string
	Cause    error
}

func (e *AppError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("[%s] %s", e.Code, e.Message)
	}
	return e.Message
}

func (e *AppError) Unwrap() error { return e.Cause }

// --- Constructors ---

func NewValidation(code, msg string) *AppError {
	return &AppError{HTTPCode: 400, Code: code, Message: msg}
}

func NewUnauthorized(code, msg string) *AppError {
	return &AppError{HTTPCode: 401, Code: code, Message: msg}
}

func NewNotFound(code, msg string) *AppError {
	return &AppError{HTTPCode: 404, Code: code, Message: msg}
}

func NewConflict(code, msg string) *AppError {
	return &AppError{HTTPCode: 409, Code: code, Message: msg}
}

func NewInternal(code, msg string, cause error) *AppError {
	return &AppError{HTTPCode: 500, Code: code, Message: msg, Cause: cause}
}

// --- Common codes (USER / AUTH / INFRA) ---

const (
	CodeValidation        = "USER_1001"
	CodeUnauthorized      = "AUTH_1001"
	CodeNotFound          = "USER_1002"
	CodeConflict          = "USER_1003"
	CodeInternal          = "INFRA_3001"
	CodeInvalidCredential = "AUTH_1002"
)

func Validation(msg string) *AppError { return NewValidation(CodeValidation, msg) }
// Unauthorized is a shorthand for a 401 AUTH error.
func Unauthorized(msg string) *AppError {
	return NewUnauthorized(CodeUnauthorized, msg)
}
func NotFound(msg string) *AppError { return NewNotFound(CodeNotFound, msg) }
func Conflict(msg string) *AppError { return NewConflict(CodeConflict, msg) }
func Internal(msg string, cause error) *AppError {
	return NewInternal(CodeInternal, msg, cause)
}

func IsAppError(err error) (*AppError, bool) {
	var ae *AppError
	if errors.As(err, &ae) {
		return ae, true
	}
	return nil, false
}
