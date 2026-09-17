package model

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type ID string

func NewID() ID { return ID(uuid.NewString()) }

func (id ID) String() string { return string(id) }

func UTC(value time.Time) time.Time { return value.UTC() }

type ValidationError struct {
	Code    string
	Field   string
	Message string
}

func (err *ValidationError) Error() string { return err.Message }

type ApplicationError struct {
	Code      string
	Message   string
	Retryable bool
}

func (err *ApplicationError) Error() string { return err.Message }

func ErrorCode(err error) string {
	var applicationError *ApplicationError
	if errors.As(err, &applicationError) {
		return applicationError.Code
	}
	var validationError *ValidationError
	if errors.As(err, &validationError) {
		return validationError.Code
	}
	return "internal_error"
}
