// Package errors carries the typed error envelope returned from every domain
// and application layer. Transport adapters (gRPC, GraphQL) map a single Kind
// to the correct status code without sniffing strings — they should call
// IsKind or As rather than type-asserting directly.
package errors

import (
	"errors"
	"fmt"
)

type Kind uint8

const (
	KindUnknown Kind = iota
	KindInvalid
	KindNotFound
	KindConflict
	KindUnauthorized
	KindForbidden
	KindInternal
)

func (k Kind) String() string {
	switch k {
	case KindInvalid:
		return "invalid"
	case KindNotFound:
		return "not_found"
	case KindConflict:
		return "conflict"
	case KindUnauthorized:
		return "unauthorized"
	case KindForbidden:
		return "forbidden"
	case KindInternal:
		return "internal"
	default:
		return "unknown"
	}
}

type Error struct {
	Kind    Kind
	Code    string
	Message string
	Wrapped error
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}

	switch {
	case e.Wrapped != nil && e.Message != "":
		return fmt.Sprintf("%s: %s: %v", e.Kind, e.Message, e.Wrapped)
	case e.Wrapped != nil:
		return fmt.Sprintf("%s: %v", e.Kind, e.Wrapped)
	case e.Message != "":
		return fmt.Sprintf("%s: %s", e.Kind, e.Message)
	default:
		return e.Kind.String()
	}
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Wrapped
}

func (e *Error) Is(target error) bool {
	if e == nil || target == nil {
		return false
	}

	if t, ok := errors.AsType[*Error](target); ok && t != nil {
		return e.Kind == t.Kind
	}

	return false
}

func NewInvalid(code, message string, wrapped error) *Error {
	return &Error{Kind: KindInvalid, Code: code, Message: message, Wrapped: wrapped}
}

func NewNotFound(code, message string, wrapped error) *Error {
	return &Error{Kind: KindNotFound, Code: code, Message: message, Wrapped: wrapped}
}

func NewConflict(code, message string, wrapped error) *Error {
	return &Error{Kind: KindConflict, Code: code, Message: message, Wrapped: wrapped}
}

func NewUnauthorized(code, message string, wrapped error) *Error {
	return &Error{Kind: KindUnauthorized, Code: code, Message: message, Wrapped: wrapped}
}

func NewForbidden(code, message string, wrapped error) *Error {
	return &Error{Kind: KindForbidden, Code: code, Message: message, Wrapped: wrapped}
}

func NewInternal(code, message string, wrapped error) *Error {
	return &Error{Kind: KindInternal, Code: code, Message: message, Wrapped: wrapped}
}

func IsKind(err error, kind Kind) bool {
	e, ok := errors.AsType[*Error](err)
	if !ok || e == nil {
		return false
	}
	return e.Kind == kind
}

func As(err error, target **Error) bool {
	return errors.As(err, target)
}
