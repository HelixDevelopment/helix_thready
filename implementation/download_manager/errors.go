// Package downloadmanager is a generic, project-agnostic multi-protocol download
// engine for the Helix Thready "Download Manager" subsystem (gap register [GAP: 6.3]).
//
// It provides a job queue with a bounded worker pool, per-job lifecycle state,
// progress reporting, retry with exponential backoff + jitter, and a completion
// callback hook. Protocol handling is pluggable behind the [Fetcher] interface:
// a real, segmented/resumable HTTP(S) fetcher is provided, plus honest
// interface stubs for ftp/smb/nfs/webdav that return [ErrNotImplemented].
package downloadmanager

import "errors"

// ErrNotImplemented is returned by fetcher stubs (FTP/SMB/NFS/WebDav) that are
// declared but not yet backed by a real transport. It is a permanent error:
// the manager will not retry it.
var ErrNotImplemented = errors.New("downloadmanager: fetcher not implemented for scheme")

// PermanentError marks an error that must not be retried by the [Manager]
// (for example an HTTP 4xx, an unsupported scheme, or a checksum mismatch).
type PermanentError struct{ Err error }

func (e *PermanentError) Error() string {
	if e.Err == nil {
		return "downloadmanager: permanent error"
	}
	return e.Err.Error()
}

// Unwrap allows errors.Is / errors.As to see the wrapped cause.
func (e *PermanentError) Unwrap() error { return e.Err }

// Permanent wraps err so that [IsPermanent] reports true. A nil err yields nil.
func Permanent(err error) error {
	if err == nil {
		return nil
	}
	return &PermanentError{Err: err}
}

// IsPermanent reports whether err should be treated as non-retryable.
// ErrNotImplemented is always permanent.
func IsPermanent(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrNotImplemented) {
		return true
	}
	var pe *PermanentError
	return errors.As(err, &pe)
}
