package errors

import (
	"errors"
	"fmt"
)

// Errores predefinidos comunes
var (
	ErrRegistryUnavailable = errors.New("registry unavailable")
	ErrInvalidImage        = errors.New("invalid image format")
	ErrConfigNotFound      = errors.New("configuration not found")
	ErrNotificationFailed  = errors.New("notification failed")
	ErrParseError          = errors.New("parse error")
	ErrNetworkError        = errors.New("network error")
	ErrAuthenticationError = errors.New("authentication error")
	ErrRateLimitExceeded   = errors.New("rate limit exceeded")
)

// Error representa un error con contexto operacional
type Error struct {
	Op  string // operación que falló
	Err error  // error subyacente
}

// Error implementa la interfaz error
func (e *Error) Error() string {
	if e.Op == "" {
		return e.Err.Error()
	}
	return fmt.Sprintf("%s: %v", e.Op, e.Err)
}

// Unwrap permite el unwrapping del error subyacente
func (e *Error) Unwrap() error {
	return e.Err
}

// Wrap crea un nuevo error con contexto operacional
func Wrap(op string, err error) error {
	if err == nil {
		return nil
	}
	return &Error{Op: op, Err: err}
}

// Wrapf crea un nuevo error con contexto operacional y formato
func Wrapf(op string, err error, format string, args ...interface{}) error {
	if err == nil {
		return nil
	}
	return &Error{
		Op:  op,
		Err: fmt.Errorf(format+": %w", append(args, err)...),
	}
}

// New crea un nuevo error con mensaje
func New(op, message string) error {
	return &Error{
		Op:  op,
		Err: errors.New(message),
	}
}

// Newf crea un nuevo error con mensaje formateado
func Newf(op, format string, args ...interface{}) error {
	return &Error{
		Op:  op,
		Err: fmt.Errorf(format, args...),
	}
}

// IsType verifica si el error es de un tipo específico
func IsType(err, target error) bool {
	return errors.Is(err, target)
}

// AsType intenta convertir el error al tipo especificado
func AsType(err error, target interface{}) bool {
	return errors.As(err, target)
}

// GetOperation extrae la operación de un error contextual
func GetOperation(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Op
	}
	return ""
}