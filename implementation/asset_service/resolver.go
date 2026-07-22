package assetservice

import (
	"context"
	"fmt"
	"io"
)

// Principal is the authenticated caller the [Authorizer] decides about. It is
// intentionally small: the identity plus the account it acts within and its
// roles. The Asset Service does not mint these — an upstream auth layer
// (digital.vasic.userservice) does.
type Principal struct {
	Subject   string   // stable user/service id
	AccountID string   // account the caller is acting within
	Roles     []string // e.g. "reader", "admin"
}

// Authorizer decides whether a principal may resolve a given asset. It is the
// injected RBAC gate; the Asset Service is deny-by-default (a nil Authorizer, or
// one that returns a non-nil error, denies). Return nil to allow, or
// [ErrForbidden] (or any error) to deny.
type Authorizer interface {
	Authorize(ctx context.Context, p Principal, a Asset) error
}

// AuthorizerFunc adapts a function to the [Authorizer] interface.
type AuthorizerFunc func(ctx context.Context, p Principal, a Asset) error

// Authorize calls f.
func (f AuthorizerFunc) Authorize(ctx context.Context, p Principal, a Asset) error {
	return f(ctx, p, a)
}

// DenyAll is the safe default Authorizer: it denies every request. Wiring it
// (or leaving the Resolver's authorizer nil) guarantees no asset is served
// until an explicit policy is injected.
var DenyAll Authorizer = AuthorizerFunc(func(context.Context, Principal, Asset) error {
	return ErrForbidden
})

// SameAccountAuthorizer allows a principal to resolve an asset only when the
// principal acts within the asset's owning account (and carries at least one
// role). It is a concrete, real RBAC policy — cross-account access is denied.
type SameAccountAuthorizer struct{}

// Authorize implements [Authorizer].
func (SameAccountAuthorizer) Authorize(_ context.Context, p Principal, a Asset) error {
	if p.AccountID == "" || len(p.Roles) == 0 {
		return fmt.Errorf("%w: unauthenticated principal", ErrForbidden)
	}
	if a.AccountID != "" && p.AccountID != a.AccountID {
		return fmt.Errorf("%w: principal account %q != asset account %q", ErrForbidden, p.AccountID, a.AccountID)
	}
	return nil
}

// ByteStore is the read side the [Resolver] depends on. Both [ContentStore] and
// [EncryptedStore] satisfy it, so a Resolver serves plaintext or encrypted
// assets identically — the sensitive-directory decrypt-only-inside-the-service
// rule (architecture §8) is just a Resolver wired to an [EncryptedStore].
type ByteStore interface {
	Get(id string) (io.ReadSeekCloser, error)
}

// Resolver maps an asset id to its bytes, gated by an [Authorizer]. It is the
// enforcement point for the "client links are NEVER direct file paths" rule:
// callers hand an opaque asset id and a [Principal] and receive verified bytes
// (or ErrForbidden / ErrNotFound) — never a path they could fetch directly.
type Resolver struct {
	index *AssetIndex
	store ByteStore
	authz Authorizer
}

// NewResolver builds a Resolver over an asset index and byte store, gated by
// authz. If authz is nil the Resolver is deny-by-default ([DenyAll]).
func NewResolver(index *AssetIndex, store ByteStore, authz Authorizer) *Resolver {
	if authz == nil {
		authz = DenyAll
	}
	return &Resolver{index: index, store: store, authz: authz}
}

// Resolve returns the raw original bytes of the asset with the given id, after
// the [Authorizer] permits p. The returned reader is integrity-verified by the
// underlying store. Errors: [ErrNotFound] (unknown id), [ErrForbidden] (denied).
func (r *Resolver) Resolve(ctx context.Context, p Principal, assetID string) (io.ReadSeekCloser, Asset, error) {
	a, ok := r.index.Get(assetID)
	if !ok {
		return nil, Asset{}, fmt.Errorf("%w: asset %q", ErrNotFound, assetID)
	}
	if err := r.authz.Authorize(ctx, p, a); err != nil {
		// Normalize any denial to ErrForbidden while preserving the cause.
		return nil, Asset{}, fmt.Errorf("%w: %v", ErrForbidden, err)
	}
	if !a.HasRaw() {
		return nil, Asset{}, fmt.Errorf("%w: asset %q has no raw content", ErrNotFound, assetID)
	}
	rc, err := r.store.Get(a.SHA256)
	if err != nil {
		return nil, Asset{}, err
	}
	return rc, a, nil
}

// ResolveRendition returns the bytes of a named rendition of the asset, subject
// to the same authorization as [Resolve]. The rendition argument is either a
// full rendition name (e.g. "video-web.mp4") or the alias "web", which resolves
// to the standard "…-web" rendition derived from the asset's original name.
func (r *Resolver) ResolveRendition(ctx context.Context, p Principal, assetID, rendition string) (io.ReadSeekCloser, Rendition, error) {
	a, ok := r.index.Get(assetID)
	if !ok {
		return nil, Rendition{}, fmt.Errorf("%w: asset %q", ErrNotFound, assetID)
	}
	if err := r.authz.Authorize(ctx, p, a); err != nil {
		return nil, Rendition{}, fmt.Errorf("%w: %v", ErrForbidden, err)
	}
	name := rendition
	if rendition == "web" {
		name = WebRenditionName(a.OriginalName)
	}
	rd, ok := a.Renditions[name]
	if !ok || rd.ContentID == "" {
		return nil, Rendition{}, fmt.Errorf("%w: rendition %q of asset %q", ErrNotFound, rendition, assetID)
	}
	rc, err := r.store.Get(rd.ContentID)
	if err != nil {
		return nil, Rendition{}, err
	}
	return rc, rd, nil
}
