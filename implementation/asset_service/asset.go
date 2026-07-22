package assetservice

import (
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Rendition is a derived variant of an asset (e.g. the web-optimized "…-web"
// version). Content points at the bytes in a [ContentStore]/[EncryptedStore] via
// the rendition's own content id.
type Rendition struct {
	Name        string // e.g. "video-web.mp4"
	ContentID   string // SHA-256 of the rendition bytes ("" until produced)
	ContentType string
	Size        int64
}

// Asset is the system-of-record record for a stored asset. Its bytes live in a
// content store keyed by SHA256; clients reference it only by ID and always
// resolve through the [Resolver] — ID is never a filesystem path.
type Asset struct {
	ID           string               // opaque asset id (never a path)
	SHA256       string               // content id of the raw original bytes
	Size         int64                // raw original size in bytes
	ContentType  string               // raw original MIME type
	OriginalName string               // e.g. "video.mp4"
	AccountID    string               // owning account (for RBAC scoping)
	CreatedAt    time.Time            // when the asset was recorded
	Renditions   map[string]Rendition // keyed by rendition name (e.g. "video-web.mp4")
}

// HasRaw reports whether the asset has raw original bytes recorded.
func (a Asset) HasRaw() bool { return a.SHA256 != "" }

// RenditionNames returns the names of the asset's renditions.
func (a Asset) RenditionNames() []string {
	names := make([]string, 0, len(a.Renditions))
	for n := range a.Renditions {
		names = append(names, n)
	}
	return names
}

// WebRenditionName derives the web-optimized rendition name from an original
// file name by inserting the "-web" suffix before the extension:
//
//	"video.mp4"      -> "video-web.mp4"
//	"clip.tar.gz"    -> "clip.tar-web.gz"   (suffix before the final extension)
//	"photo.jpeg"     -> "photo-web.jpeg"
//	"noext"          -> "noext-web"
//	"archive/a.mp3"  -> "a-web.mp3"          (directory dropped; name only)
//
// This is the "…-web" convention from the architecture (§7): raw preserved, a
// web-optimized sibling produced with the suffix before the extension.
func WebRenditionName(original string) string {
	return webRenditionName(original, "web")
}

// webRenditionName is the general form used by [WebRenditionName]; suffix is the
// token inserted before the extension ("web").
func webRenditionName(original, suffix string) string {
	base := filepath.Base(original)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	return stem + "-" + suffix + ext
}

// AssetIndex is a concurrency-safe in-memory registry mapping asset id -> Asset.
// It is the minimal system-of-record the [Resolver] consults; a production
// deployment would back this with the relational SoR (architecture §7). The
// index deliberately exposes no method to obtain a filesystem path.
type AssetIndex struct {
	mu     sync.RWMutex
	assets map[string]Asset
}

// NewAssetIndex returns an empty index.
func NewAssetIndex() *AssetIndex {
	return &AssetIndex{assets: make(map[string]Asset)}
}

// Put records (or replaces) an asset by its ID.
func (ix *AssetIndex) Put(a Asset) {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	ix.assets[a.ID] = a
}

// Get returns the asset for id and whether it was found.
func (ix *AssetIndex) Get(id string) (Asset, bool) {
	ix.mu.RLock()
	defer ix.mu.RUnlock()
	a, ok := ix.assets[id]
	return a, ok
}
