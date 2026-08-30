package ghanageo

import "fmt"

type Error struct {
	Status    int            `json:"-"`
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	RequestID string         `json:"requestId,omitempty"`
	Details   map[string]any `json:"details,omitempty"`
	Docs      string         `json:"docs,omitempty"`
}

func (e *Error) Error() string {
	if e.RequestID != "" {
		return fmt.Sprintf("ghanageo: %s (%s, request %s)", e.Message, e.Code, e.RequestID)
	}
	return fmt.Sprintf("ghanageo: %s (%s)", e.Message, e.Code)
}

func (e *Error) Is(target error) bool {
	want, ok := target.(*Error)
	return ok && want.Code != "" && e.Code == want.Code
}

var (
	ErrInvalidArgument    = &Error{Code: "INVALID_ARGUMENT"}
	ErrInvalidCoordinates = &Error{Code: "INVALID_COORDINATES"}
	ErrRadiusOutOfRange   = &Error{Code: "RADIUS_OUT_OF_RANGE"}
	ErrQueryTooShort      = &Error{Code: "QUERY_TOO_SHORT"}
	ErrPayloadTooLarge    = &Error{Code: "PAYLOAD_TOO_LARGE"}
	ErrUnauthenticated    = &Error{Code: "UNAUTHENTICATED"}
	ErrKeyRevoked         = &Error{Code: "KEY_REVOKED"}
	ErrPermissionDenied   = &Error{Code: "PERMISSION_DENIED"}
	ErrOriginNotAllowed   = &Error{Code: "ORIGIN_NOT_ALLOWED"}
	ErrNotFound           = &Error{Code: "NOT_FOUND"}
	ErrResourceGone       = &Error{Code: "RESOURCE_GONE"}
	ErrRateLimited        = &Error{Code: "RATE_LIMIT_EXCEEDED"}
	ErrQuotaExceeded      = &Error{Code: "QUOTA_EXCEEDED"}
	ErrQueryTooComplex    = &Error{Code: "QUERY_TOO_COMPLEX"}
	ErrDeadlineExceeded   = &Error{Code: "DEADLINE_EXCEEDED"}
	ErrInternal           = &Error{Code: "INTERNAL"}
	ErrRepeatedCursor     = &Error{Code: "INVALID_CURSOR", Message: "server returned a repeated pagination cursor"}
	ErrDownloadTooLarge   = &Error{Code: "DOWNLOAD_TOO_LARGE", Message: "dataset artifact exceeds configured maximum size"}
	ErrChecksumMismatch   = &Error{Code: "CHECKSUM_MISMATCH", Message: "dataset artifact checksum did not match"}
)
