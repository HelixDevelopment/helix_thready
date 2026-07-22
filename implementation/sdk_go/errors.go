package thready

import (
	"errors"
	"fmt"
)

// ErrInsecureTransport is returned instead of attaching a credential (an
// "Authorization: Bearer …" or "X-API-Key: …" header) to a request that would
// travel over plaintext http to a NON-loopback host. Sending a bearer token or
// API key in the clear to a remote origin would expose it to any on-path
// observer, so the SDK refuses by default. https (any host) and http to a
// loopback host (127.0.0.1, ::1, localhost) are always allowed; set
// Config.AllowInsecureHTTP to opt out of the refusal on trusted networks.
//
// Recover it with errors.Is(err, ErrInsecureTransport).
var ErrInsecureTransport = errors.New("thready: refusing to send credentials over plaintext http to a non-loopback host; use https or set Config.AllowInsecureHTTP")

// Code is a stable, machine-readable error code. The values mirror the canonical
// taxonomy served by the Helix Thready REST /v1 gateway (see the sibling
// implementation/rest_gateway/errors.go and docs/.../api/error-model.md), which
// maps 1:1 with the Connect/gRPC canonical codes so a single client-side error
// handler works across REST and the event/DTO plane.
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

// Detail is a structured, machine-usable reason attached to an error, matching
// the gateway's error `details[]` entries.
type Detail struct {
	Field  string `json:"field,omitempty"`
	Issue  string `json:"issue,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// APIError is the typed error surfaced for every non-2xx response. It is decoded
// from the gateway's canonical failure envelope:
//
//	{"error":{"code","message","status","request_id","trace_id","details":[...]}}
//
// Callers use errors.As to recover it and branch on Code / Status.
type APIError struct {
	Code       Code     `json:"code"`
	Message    string   `json:"message"`
	Status     int      `json:"status"`
	RequestID  string   `json:"request_id"`
	TraceID    string   `json:"trace_id"`
	RetryAfter *int     `json:"retry_after,omitempty"`
	Details    []Detail `json:"details,omitempty"`
}

// errorEnvelope is the wire wrapper: {"error": {...}}.
type errorEnvelope struct {
	Error APIError `json:"error"`
}

// Error implements the error interface.
func (e *APIError) Error() string {
	if e.RequestID != "" {
		return fmt.Sprintf("thready: %s (%d): %s [request_id=%s]", e.Code, e.Status, e.Message, e.RequestID)
	}
	return fmt.Sprintf("thready: %s (%d): %s", e.Code, e.Status, e.Message)
}

// Retryable reports whether the error's code is one the SDK may transparently
// retry (rate limiting or a transient downstream outage).
func (e *APIError) Retryable() bool {
	switch e.Code {
	case CodeRateLimited, CodeUnavailable, CodeDeadlineExceeded:
		return true
	default:
		return false
	}
}
