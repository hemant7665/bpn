package apperrors

import "fmt"

type ValidationError struct {
	Message string
}

func (e ValidationError) Error() string { return e.Message }

type NotFoundError struct {
	Message string
}

func (e NotFoundError) Error() string { return e.Message }

type InternalError struct {
	Message string
	Cause   error
}

func (e InternalError) Error() string {
	if e.Cause == nil {
		return e.Message
	}
	return fmt.Sprintf("%s: %v", e.Message, e.Cause)
}

func NewValidation(msg string) error { return ValidationError{Message: msg} }
func NewNotFound(msg string) error   { return NotFoundError{Message: msg} }
func NewInternal(msg string, cause error) error {
	return InternalError{Message: msg, Cause: cause}
}
