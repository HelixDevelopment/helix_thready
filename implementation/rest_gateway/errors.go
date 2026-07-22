package gateway

import (
	"encoding/json"
	"net/http"
)

// Code is a stable, machine-readable error code. The values mirror the
// canonical taxonomy in docs/public/research/mvp/api/error-model.md, which is
// itself chosen to map 1:1 with the Connect/gRPC canonical codes so that a
// single application error handler works across REST and the event/DTO plane.
type Code string

const (
	CodeInvalidArgument  Code = "invalid_argument"
	CodeUnprocessable    Code = "unprocessable"
	CodeUnauthenticated  Code = "unauthenticated"
	CodePermissionDenied Code = "permission_denied"
	CodeNotFound         Code = "not_found"
	CodeAlreadyExists    Code = "already_exists"
	CodeConflict         Code = "conflict"
	CodeFailedPrecond    Code = "failed_precondition"
	CodeRateLimited      Code = "rate_limited"
	CodeDeadlineExceeded Code = "deadline_exceeded"
	CodeUnavailable      Code = "unavailable"
	CodeInternal         Code = "internal"
)

// statusForCode is the authoritative code -> HTTP status mirror.
func statusForCode(c Code) int {
	switch c {
	case CodeInvalidArgument:
		return http.StatusBadRequest
	case CodeUnprocessable:
		return http.StatusUnprocessableEntity
	case CodeUnauthenticated:
		return http.StatusUnauthorized
	case CodePermissionDenied:
		return http.StatusForbidden
	case CodeNotFound:
		return http.StatusNotFound
	case CodeAlreadyExists:
		return http.StatusConflict
	case CodeConflict:
		return http.StatusConflict
	case CodeFailedPrecond:
		return http.StatusPreconditionFailed
	case CodeRateLimited:
		return http.StatusTooManyRequests
	case CodeDeadlineExceeded:
		return http.StatusGatewayTimeout
	case CodeUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// Detail is a structured, machine-usable reason attached to an error.
type Detail struct {
	Field  string `json:"field,omitempty"`
	Issue  string `json:"issue,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// errorBody is the single failure envelope returned for every non-2xx response.
// {"error":{"code","message","request_id",...}} — request_id is always present
// so a report maps back to logs/traces.
type errorBody struct {
	Error errorEnvelope `json:"error"`
}

type errorEnvelope struct {
	Code      Code     `json:"code"`
	Message   string   `json:"message"`
	RequestID string   `json:"request_id"`
	Status    int      `json:"status"`
	TraceID   string   `json:"trace_id"`
	Details   []Detail `json:"details,omitempty"`
}

// CodedError is an error that carries a stable, machine-readable Code. ANY
// Service implementation — including ones in sibling packages that cannot see
// the gateway's internal carrier — can return a CodedError so the response
// writer maps it to the correct HTTP status + envelope (via the code->status
// table) instead of collapsing it to a blanket 500. Build one with NewError, or
// implement this interface directly. ErrorCode returns one of the exported Code
// constants rendered as a string (e.g. string(CodeNotFound)).
type CodedError interface {
	error
	ErrorCode() string
}

// apiError is the canonical CodedError carrier that a handler can return /
// panic-free propagate to be rendered through the single envelope.
type apiError struct {
	Code    Code
	Message string
	Details []Detail
}

// interface guard: *apiError is a CodedError.
var _ CodedError = (*apiError)(nil)

func (e *apiError) Error() string { return string(e.Code) + ": " + e.Message }

// ErrorCode satisfies CodedError, exposing the stable code as a string.
func (e *apiError) ErrorCode() string { return string(e.Code) }

// NewError is the EXPORTED constructor for a coded Service error. A Service can
// return NewError(CodeNotFound, "…") and the gateway maps it to a 404 + the
// canonical error envelope (see writeServiceError). It returns the error
// interface; the concrete carrier is the same *apiError newError produces.
func NewError(code Code, message string, details ...Detail) error {
	return &apiError{Code: code, Message: message, Details: details}
}

// newError is the internal constructor returning the concrete *apiError, used by
// call sites that need the concrete type (e.g. decodeJSON). It is an unexported
// alias of NewError over the same carrier.
func newError(code Code, message string, details ...Detail) *apiError {
	return &apiError{Code: code, Message: message, Details: details}
}

// writeError renders an error through the canonical envelope. The request_id /
// trace_id are pulled from the request context (set by the request-id
// middleware) so every failure is correlatable.
func writeError(w http.ResponseWriter, r *http.Request, code Code, message string, details ...Detail) {
	rid := requestIDFrom(r.Context())
	status := statusForCode(code)
	body := errorBody{Error: errorEnvelope{
		Code:      code,
		Message:   message,
		RequestID: rid,
		Status:    status,
		TraceID:   rid,
		Details:   details,
	}}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// writeJSON renders a success payload as JSON with the given status.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
