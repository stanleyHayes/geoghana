// Package apierr is the single source of stable machine error codes.
// Every transport maps from these; no handler invents a code (Spec 19).
package apierr

import (
	"errors"
	"fmt"
	"net/http"
)

type Code string

const (
	InvalidArgument   Code = "INVALID_ARGUMENT"
	InvalidCoordinate Code = "INVALID_COORDINATES"
	RadiusOutOfRange  Code = "RADIUS_OUT_OF_RANGE"
	QueryTooShort     Code = "QUERY_TOO_SHORT"
	PayloadTooLarge   Code = "PAYLOAD_TOO_LARGE"
	Unauthenticated   Code = "UNAUTHENTICATED"
	KeyRevoked        Code = "KEY_REVOKED"
	PermissionDenied  Code = "PERMISSION_DENIED"
	OriginNotAllowed  Code = "ORIGIN_NOT_ALLOWED"
	NotFound          Code = "NOT_FOUND"
	ResourceGone      Code = "RESOURCE_GONE"
	RateLimitExceeded Code = "RATE_LIMIT_EXCEEDED"
	QuotaExceeded     Code = "QUOTA_EXCEEDED"
	QueryTooComplex   Code = "QUERY_TOO_COMPLEX"
	DeadlineExceeded  Code = "DEADLINE_EXCEEDED"
	Internal          Code = "INTERNAL"
)

var httpStatus = map[Code]int{
	InvalidArgument:   http.StatusBadRequest,
	InvalidCoordinate: http.StatusBadRequest,
	RadiusOutOfRange:  http.StatusBadRequest,
	QueryTooShort:     http.StatusBadRequest,
	QueryTooComplex:   http.StatusBadRequest,
	PayloadTooLarge:   http.StatusRequestEntityTooLarge,
	Unauthenticated:   http.StatusUnauthorized,
	KeyRevoked:        http.StatusUnauthorized,
	PermissionDenied:  http.StatusForbidden,
	OriginNotAllowed:  http.StatusForbidden,
	NotFound:          http.StatusNotFound,
	ResourceGone:      http.StatusGone,
	RateLimitExceeded: http.StatusTooManyRequests,
	QuotaExceeded:     http.StatusTooManyRequests,
	DeadlineExceeded:  http.StatusGatewayTimeout,
	Internal:          http.StatusInternalServerError,
}

func (c Code) HTTPStatus() int {
	if s, ok := httpStatus[c]; ok {
		return s
	}
	return http.StatusInternalServerError
}

// DocsURL points at the public error page every response links to.
func (c Code) DocsURL() string { return "/docs/errors/" + string(c) }

// Error carries a stable code plus optional structured details.
type Error struct {
	Code    Code
	Message string
	Details map[string]any
	cause   error
}

func (e *Error) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}
func (e *Error) Unwrap() error { return e.cause }

func New(c Code, msg string) *Error { return &Error{Code: c, Message: msg} }

func Wrap(c Code, msg string, cause error) *Error {
	return &Error{Code: c, Message: msg, cause: cause}
}

func (e *Error) WithDetail(k string, v any) *Error {
	if e.Details == nil {
		e.Details = map[string]any{}
	}
	e.Details[k] = v
	return e
}

// From maps any error to an *Error, defaulting to INTERNAL so an unmapped
// failure can never leak an implementation detail to a caller.
func From(err error) *Error {
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	return Wrap(Internal, "An unexpected error occurred.", err)
}
