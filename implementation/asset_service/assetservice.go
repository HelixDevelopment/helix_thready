// Package assetservice is the decoupled Asset Service core for Helix Thready
// (gap register [BUILD-NEW: Asset Service]). It stores, secures and serves the
// bytes that the Download Manager fetches, keeping the fetch-vs-store concerns
// separate (architecture: docs/public/research/mvp/architecture/asset-and-download.md).
//
// The core provides:
//
//   - [FileSource]: a multi-protocol access seam (Open / Stat / OpenSeekable for
//     HTTP Range). [LocalSource] (os) and [HTTPSource] (net/http) are real;
//     FTP/SMB/NFS/WebDAV are honest [ErrNotImplemented] stubs — the documented
//     reuse points for digital.vasic.filesystem.
//   - [ContentStore]: content-addressed storage keyed by SHA-256 with
//     integrity-verify-on-read (a byte that does not hash to its id is refused,
//     never silently returned).
//   - [EncryptedStore]: an AES-256-GCM wrapper around a [ContentStore] with a
//     per-asset nonce; a wrong key produces an authentication failure, never
//     silent garbage.
//   - [Asset]/[AssetIndex]/[Resolver]: an asset id -> bytes mapping gated by an
//     injected [Authorizer] (deny by default). Client links are NEVER direct
//     file paths — they resolve through the service.
//   - [Handler]: an http.ServeContent-compatible handler so byte-range requests
//     (HTTP 206 Partial Content) work over the RBAC-gated resolver.
//
// The module is self-contained: standard library only, no in-house imports.
package assetservice

import "errors"

// Errors returned across the package. They are sentinel values so callers can
// branch with errors.Is and HTTP handlers can map them to status codes.
var (
	// ErrNotImplemented is returned by the FTP/SMB/NFS/WebDAV [FileSource]
	// stubs that are declared but not yet backed by a real transport. It marks
	// the digital.vasic.filesystem reuse seam ([BUILD-NEW], not-yet-wired).
	ErrNotImplemented = errors.New("assetservice: source not implemented for scheme")

	// ErrIntegrity is returned when stored bytes do not hash to the content id
	// they are stored under — i.e. tamper / bit-rot detection on read.
	ErrIntegrity = errors.New("assetservice: integrity check failed (bytes do not match content id)")

	// ErrNotFound is returned when a content id or asset id is unknown.
	ErrNotFound = errors.New("assetservice: not found")

	// ErrForbidden is returned when the injected [Authorizer] denies a
	// principal access to an asset. It is the 403-equivalent.
	ErrForbidden = errors.New("assetservice: access denied")

	// ErrDecrypt is returned when AES-256-GCM authentication fails on read —
	// a wrong key or tampered ciphertext. It is never a silent partial result.
	ErrDecrypt = errors.New("assetservice: decryption/authentication failed")

	// ErrBadKey is returned when an encryption key is not exactly 32 bytes
	// (AES-256).
	ErrBadKey = errors.New("assetservice: encryption key must be 32 bytes (AES-256)")

	// ErrBadContentID is returned when a content id is not a well-formed
	// lowercase hex SHA-256 (64 hex chars).
	ErrBadContentID = errors.New("assetservice: malformed content id")
)
