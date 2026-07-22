package assetservice

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"
)

// seedAsset stores content and registers an asset owned by accountID, returning
// the populated index and store.
func seedAsset(t *testing.T, assetID, accountID string, content []byte) (*AssetIndex, *ContentStore, Asset) {
	t.Helper()
	cs := newStore(t)
	id, size, err := cs.Put(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	a := Asset{
		ID:           assetID,
		SHA256:       id,
		Size:         size,
		ContentType:  "application/octet-stream",
		OriginalName: "file.bin",
		AccountID:    accountID,
		CreatedAt:    time.Now(),
	}
	ix := NewAssetIndex()
	ix.Put(a)
	return ix, cs, a
}

func TestResolverDenyByDefault(t *testing.T) {
	ix, cs, _ := seedAsset(t, "asset-1", "acct-A", []byte("secret bytes"))
	// nil authorizer => deny by default.
	r := NewResolver(ix, cs, nil)
	_, _, err := r.Resolve(context.Background(), Principal{Subject: "u1", AccountID: "acct-A", Roles: []string{"reader"}}, "asset-1")
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("deny-by-default: err = %v, want ErrForbidden", err)
	}
}

func TestResolverForbidsCrossAccount(t *testing.T) {
	ix, cs, _ := seedAsset(t, "asset-1", "acct-A", []byte("account A bytes"))
	r := NewResolver(ix, cs, SameAccountAuthorizer{})
	// Principal belongs to a DIFFERENT account.
	_, _, err := r.Resolve(context.Background(), Principal{Subject: "u2", AccountID: "acct-B", Roles: []string{"reader"}}, "asset-1")
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("cross-account: err = %v, want ErrForbidden", err)
	}
}

func TestResolverAllowsAuthorized(t *testing.T) {
	content := []byte("authorized principal may read these bytes")
	ix, cs, _ := seedAsset(t, "asset-1", "acct-A", content)
	r := NewResolver(ix, cs, SameAccountAuthorizer{})
	rc, a, err := r.Resolve(context.Background(), Principal{Subject: "u1", AccountID: "acct-A", Roles: []string{"reader"}}, "asset-1")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	defer rc.Close()
	if a.ID != "asset-1" {
		t.Fatalf("asset id = %q", a.ID)
	}
	got, _ := io.ReadAll(rc)
	if !bytes.Equal(got, content) {
		t.Fatalf("bytes = %q, want %q", got, content)
	}
}

func TestResolverUnknownAsset(t *testing.T) {
	ix := NewAssetIndex()
	cs := newStore(t)
	r := NewResolver(ix, cs, AuthorizerFunc(func(context.Context, Principal, Asset) error { return nil }))
	if _, _, err := r.Resolve(context.Background(), Principal{}, "ghost"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown asset: err = %v, want ErrNotFound", err)
	}
}

func TestResolverRendition(t *testing.T) {
	cs := newStore(t)
	raw := []byte("raw original video bytes")
	web := []byte("smaller web-optimized bytes")
	rawID, _, err := cs.Put(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("Put raw: %v", err)
	}
	webID, _, err := cs.Put(bytes.NewReader(web))
	if err != nil {
		t.Fatalf("Put web: %v", err)
	}
	webName := WebRenditionName("video.mp4")
	a := Asset{
		ID:           "asset-1",
		SHA256:       rawID,
		OriginalName: "video.mp4",
		AccountID:    "acct-A",
		Renditions: map[string]Rendition{
			webName: {Name: webName, ContentID: webID, ContentType: "video/mp4", Size: int64(len(web))},
		},
	}
	ix := NewAssetIndex()
	ix.Put(a)
	r := NewResolver(ix, cs, SameAccountAuthorizer{})

	rc, rd, err := r.ResolveRendition(context.Background(), Principal{AccountID: "acct-A", Roles: []string{"reader"}}, "asset-1", "web")
	if err != nil {
		t.Fatalf("ResolveRendition: %v", err)
	}
	defer rc.Close()
	if rd.Name != "video-web.mp4" {
		t.Fatalf("rendition name = %q, want video-web.mp4", rd.Name)
	}
	got, _ := io.ReadAll(rc)
	if !bytes.Equal(got, web) {
		t.Fatalf("rendition bytes = %q, want %q", got, web)
	}
}

// TestResolverServesEncryptedStore proves a Resolver over an EncryptedStore
// hands back decrypted bytes only after authorization (the sensitive-directory
// decrypt-only-inside-the-service rule).
func TestResolverServesEncryptedStore(t *testing.T) {
	cs := newStore(t)
	es, err := NewEncryptedStore(cs, mustKey(t))
	if err != nil {
		t.Fatalf("NewEncryptedStore: %v", err)
	}
	plain := []byte("sensitive contract bytes, decrypt only inside the service")
	id, size, err := es.Put(bytes.NewReader(plain))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	ix := NewAssetIndex()
	ix.Put(Asset{ID: "sensitive-1", SHA256: id, Size: size, AccountID: "acct-A", OriginalName: "contract.pdf"})
	r := NewResolver(ix, es, SameAccountAuthorizer{})

	// Denied for the wrong account.
	if _, _, err := r.Resolve(context.Background(), Principal{AccountID: "acct-B", Roles: []string{"reader"}}, "sensitive-1"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("cross-account on encrypted asset: err = %v, want ErrForbidden", err)
	}
	// Allowed + decrypted for the right account.
	rc, _, err := r.Resolve(context.Background(), Principal{AccountID: "acct-A", Roles: []string{"reader"}}, "sensitive-1")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	defer rc.Close()
	got, _ := io.ReadAll(rc)
	if !bytes.Equal(got, plain) {
		t.Fatalf("decrypted bytes = %q, want %q", got, plain)
	}
}
