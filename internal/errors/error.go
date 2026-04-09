// Package errors provides centralized application errors (BluePrint-style CustomError + stable codes).
package errors

import (
	stderrors "errors"
	"fmt"
)

// CustomError is a structured application error (aligned with blueprint-serverless/internal/errors).
type CustomError struct {
	Message  string `json:"message"`
	Code     string `json:"code"`
	HTTPCode int    `json:"httpCode"`
	Details  any    `json:"details,omitempty"`
	Cause    error  `json:"-"`
}

// AppError is an alias for CustomError for existing call sites and tests.
type AppError = CustomError

func (e *CustomError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("[%s] %s", e.Code, e.Message)
	}
	return e.Message
}

func (e *CustomError) Unwrap() error { return e.Cause }

// Is supports errors.Is for predefined *CustomError variables.
func (e *CustomError) Is(target error) bool {
	t, ok := target.(*CustomError)
	if !ok {
		return false
	}
	return e.Code == t.Code && e.Message == t.Message && e.HTTPCode == t.HTTPCode
}

// NewCustomError creates a new custom error (optional first detail becomes Details).
func NewCustomError(httpCode int, code, message string, details ...any) *CustomError {
	err := &CustomError{
		HTTPCode: httpCode,
		Code:     code,
		Message:  message,
	}
	if len(details) > 0 {
		err.Details = details[0]
	}
	return err
}

// New creates a custom error without details (same as blueprint errors.New).
func New(httpCode int, code, message string) *CustomError {
	return &CustomError{
		HTTPCode: httpCode,
		Code:     code,
		Message:  message,
	}
}

// NewInternal is a 500 error with an optional wrapped cause.
func NewInternal(code, message string, cause error) *CustomError {
	return &CustomError{
		HTTPCode: 500,
		Code:     code,
		Message:  message,
		Cause:    cause,
	}
}

// Wrap wraps an error with a message prefix.
func Wrap(err error, message string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", message, err)
}

// --- Stable code strings ---

const (
	CodeValidation        = "USER_1001"
	CodeUnauthorized      = "AUTH_1001"
	CodeNotFound          = "USER_1002"
	CodeConflict          = "USER_1003"
	CodeInternal          = "INFRA_3001"
	CodeInvalidCredential = "AUTH_1002"

	CodeImportJobNotFound  = "IMPORT_2001"
	CodeImportInvalidState = "IMPORT_2002"
	CodeImportS3Missing    = "IMPORT_2003"
	CodeImportQueue        = "IMPORT_2004"
	CodeImportValidation   = "IMPORT_2005"
	CodeImportInternal     = "IMPORT_2006"
)

// --- Pre-defined errors (BluePrint-style) ---

var (
	ErrValidationFailed = NewCustomError(400, CodeValidation, "One or more validation errors occurred.")
	ErrInvalidRequest   = NewCustomError(400, CodeValidation, "The request was malformed or invalid.")

	ErrUnauthorized       = NewCustomError(401, CodeUnauthorized, "Authentication is required and has failed or has not yet been provided.")
	ErrInvalidCredentials = NewCustomError(401, CodeInvalidCredential, "Invalid credentials.")

	ErrNotFound = NewCustomError(404, CodeNotFound, "The requested resource could not be found.")
	ErrConflict = NewCustomError(409, CodeConflict, "The request could not be completed due to a conflict with the current state of the resource.")

	ErrInternalServer = NewCustomError(500, CodeInternal, "An unexpected error occurred on the server.")

	ErrImportJobNotFound  = NewCustomError(404, CodeImportJobNotFound, "Import job not found.")
	ErrImportInvalidState = NewCustomError(409, CodeImportInvalidState, "Import job is in an invalid state for this operation.")
	ErrImportS3Missing    = NewCustomError(400, CodeImportS3Missing, "CSV file not found in storage.")
	ErrImportQueueFailed  = NewCustomError(500, CodeImportQueue, "Failed to enqueue import job.")
	ErrImportValidation   = NewCustomError(400, CodeImportValidation, "Invalid import request.")
	ErrImportInternal     = NewCustomError(500, CodeImportInternal, "An unexpected import error occurred.")

	// Database bootstrap
	ErrDatabaseURLNotSet  = New(503, CodeInternal, "database URL is not set")
	ErrDatabaseURLInvalid = New(500, CodeInternal, "database URL is invalid")

	// User write path — stable pointer for errors.Is (see user service CreateUser).
	ErrEmailAlreadyExists = NewConflict(CodeConflict, "user with this email already exists for this tenant")
)

// PlainMessage returns an error whose Error() text is exactly msg (no [CODE] prefix).
// Use for values embedded in other payloads (e.g. CSV row validation reasons).
func PlainMessage(msg string) *CustomError {
	return &CustomError{Message: msg}
}

// --- Low-level constructors (code + message) ---

func NewValidation(code, msg string) *CustomError {
	return New(400, code, msg)
}

func NewUnauthorized(code, msg string) *CustomError {
	return New(401, code, msg)
}

func NewNotFound(code, msg string) *CustomError {
	return New(404, code, msg)
}

func NewConflict(code, msg string) *CustomError {
	return New(409, code, msg)
}

// --- Shorthand helpers (dynamic messages, same codes as before) ---

func Validation(msg string) *CustomError {
	return NewValidation(CodeValidation, msg)
}

func Unauthorized(msg string) *CustomError {
	return NewUnauthorized(CodeUnauthorized, msg)
}

func NotFound(msg string) *CustomError {
	return NewNotFound(CodeNotFound, msg)
}

func Conflict(msg string) *CustomError {
	return NewConflict(CodeConflict, msg)
}

func Internal(msg string, cause error) *CustomError {
	return NewInternal(CodeInternal, msg, cause)
}

func IsAppError(err error) (*CustomError, bool) {
	var ce *CustomError
	if stderrors.As(err, &ce) {
		return ce, true
	}
	return nil, false
}

// --- Import job helpers ---

func ImportJobNotFound(msg string) *CustomError {
	return NewNotFound(CodeImportJobNotFound, msg)
}

func ImportInvalidState(msg string) *CustomError {
	return NewConflict(CodeImportInvalidState, msg)
}

func ImportS3Missing(msg string) *CustomError {
	return NewValidation(CodeImportS3Missing, msg)
}

func ImportQueue(msg string, cause error) *CustomError {
	return NewInternal(CodeImportQueue, msg, cause)
}

func ImportValidation(msg string) *CustomError {
	return NewValidation(CodeImportValidation, msg)
}

func ImportInternal(msg string, cause error) *CustomError {
	return NewInternal(CodeImportInternal, msg, cause)
}
