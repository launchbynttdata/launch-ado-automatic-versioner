package exitcodes

import (
	"errors"
	"fmt"
)

const (
	Success            = 0
	ConfigError        = 1
	ADOAPIError        = 2
	SemanticValidation = 3
)

// SemanticError wraps semantic validation failures for exit code mapping.
type SemanticError struct {
	Message string
	Cause   error
}

func (e *SemanticError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return "semantic validation error"
}

func (e *SemanticError) Unwrap() error {
	return e.Cause
}

// NewSemanticError constructs a semantic validation error.
func NewSemanticError(message string, cause error) *SemanticError {
	return &SemanticError{Message: message, Cause: cause}
}

// CodeFor returns the exit code for err.
func CodeFor(err error) int {
	if err == nil {
		return Success
	}
	var semantic *SemanticError
	if errors.As(err, &semantic) {
		return SemanticValidation
	}
	if errors.Is(err, ErrADOAPI) {
		return ADOAPIError
	}
	if errors.Is(err, ErrConfig) {
		return ConfigError
	}
	return ConfigError
}

var (
	ErrConfig = errors.New("configuration error")
	ErrADOAPI = errors.New("ado api error")
)

// WrapADOAPI annotates an ADO API failure.
func WrapADOAPI(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w: %w", ErrADOAPI, err)
}

// WrapConfig annotates a configuration failure.
func WrapConfig(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w: %w", ErrConfig, err)
}
