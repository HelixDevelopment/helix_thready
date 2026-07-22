# digital.vasic.assetservice

The decoupled **Asset Service** core for Helix Thready (gap register
`[BUILD-NEW: Asset Service]`). It **stores, secures and serves** the bytes that
the Download Manager fetches — the store/serve half of the fetch-vs-store
separation in the architecture
(`docs/public/research/mvp/architecture/asset-and-download.md` §1, §2, §7, §8).

- **Module:** `digital.vasic.assetservice`
- **Go:** 1.26, **standard library only** (no external dependencies).
- **Self-contained:** imports no in-house modules. The FTP/SMB/NFS/WebDAV
  `FileSource` stubs are the documented reuse points for
  `digital.vasic.filesystem`; RBAC principals come from
  `digital.vasic.userservice` — neither is imported here.

## The rule: client links are NEVER direct file paths

Every client reference is an **opaque asset id**, resolved through the service:

```
GET /v1/assets/{id}         → RBAC-gated, integrity-verified, Range-capable
GET /v1/assets/{id}/web     → the "…-web" optimized rendition
```

The on-disk blob path is unexported (`ContentStore.blobPath`) and reachable
through **no** public API. Resolution goes `asset id → Authorizer check →
content id → verified bytes`. An unauthorized principal gets `403`; there is no
API that returns a filesystem location. `TestServeNoDirectPath` guards this.

## What is real vs. stub

| Capability | Status |
|------------|--------|
| Content-addressed store keyed by sha256 | **REAL** — `ContentStore` |
| Integrity-verify-on-read (tamper detection) | **REAL** — mismatch → `ErrIntegrity`, no bytes returned |
| Content-hash dedup (identical bytes stored once) | **REAL** |
| AES-256-GCM encryption at rest, per-asset nonce | **REAL** — `EncryptedStore` |
| Wrong-key rejection (GCM auth, never silent garbage) | **REAL** — `ErrDecrypt` |
| Local filesystem source | **REAL** — `LocalSource` (os), path-traversal confined |
| Remote HTTP(S) source | **REAL** — `HTTPSource` (net/http) |
| HTTP Range / 206 serving | **REAL** — `Handler` via `http.ServeContent` |
| RBAC-gated resolution, deny-by-default | **REAL** — `Resolver` + `Authorizer` |
| `…-web` rendition-name derivation | **REAL** — `WebRenditionName` |
| FTP / SMB / NFS / WebDAV source | **STUB** — `NewStubSource`, returns `ErrNotImplemented` (`[BUILD-NEW]`, never faked) |
| HLS/DASH manifests + transcoder | **out of core scope** — `[OPEN: ASSET-2]`, not claimed |

## Public API

```go
// --- Content-addressed store ---
cs, _ := assetservice.NewContentStore("/var/lib/thready/assets")
id, size, _ := cs.Put(reader)          // id == hex sha256(bytes); dedups identical content
rc, err := cs.Get(id)                   // ReadSeekCloser, integrity-verified; ErrIntegrity on tamper
info, _ := cs.Stat(id)                  // ContentInfo{ID, Size}

// --- Encryption at rest (AES-256-GCM) wrapping a ContentStore ---
key := make([]byte, 32)                  // AES-256; 32 bytes required (else ErrBadKey)
es, _ := assetservice.NewEncryptedStore(cs, key)
pid, _, _ := es.Put(reader)              // pid == sha256(plaintext); ciphertext on disk
plain, err := es.Get(pid)                // decrypts; wrong key → ErrDecrypt

// --- Multi-protocol access (FileSource) ---
local, _ := assetservice.NewLocalSource("/srv/assets")   // os
http := assetservice.NewHTTPSource(nil)                  // net/http remote fetch
ftp  := assetservice.NewStubSource("ftp")                // ErrNotImplemented (filesystem seam)
rsc, _ := local.OpenSeekable(ctx, "video.mp4")           // io.ReadSeekCloser for Range

// --- Assets, renditions, RBAC resolution ---
ix := assetservice.NewAssetIndex()
ix.Put(assetservice.Asset{
    ID: "9c1e…", SHA256: id, Size: size, ContentType: "video/mp4",
    OriginalName: "video.mp4", AccountID: "acct-A",
    Renditions: map[string]assetservice.Rendition{
        assetservice.WebRenditionName("video.mp4"): { // "video-web.mp4"
            Name: "video-web.mp4", ContentID: webID, ContentType: "video/mp4",
        },
    },
})
r := assetservice.NewResolver(ix, cs, assetservice.SameAccountAuthorizer{}) // deny-by-default if authz is nil
rc, asset, err := r.Resolve(ctx, principal, "9c1e…")  // ErrForbidden / ErrNotFound / bytes

// --- HTTP serving (Range-capable, never a direct path) ---
h := assetservice.NewHandler(r, func(req *http.Request) (assetservice.Principal, error) {
    return verifyBearer(req) // your auth layer (digital.vasic.userservice)
})
mux.Handle("/v1/assets/", h)   // GET {id} and GET {id}/web; ServeContent handles Range → 206
```

### RBAC (deny by default)

`Resolver` gates every resolution through an injected `Authorizer`:

- A `nil` authorizer, or the built-in `DenyAll`, **denies everything** — nothing
  is served until an explicit policy is wired.
- `SameAccountAuthorizer` allows a principal only within the asset's owning
  account and only when it carries a role; cross-account access → `ErrForbidden`.
- Sensitive assets: point the `Resolver` at an `EncryptedStore` and the bytes
  are only ever handed out **decrypted, after** the RBAC check — the
  "specially encrypted directory decrypts only inside the Asset Service" rule.

### `…-web` rendition convention

`WebRenditionName(original)` inserts `-web` before the extension
(`video.mp4 → video-web.mp4`, `noext → noext-web`). The raw original is always
preserved (`Asset.SHA256` / `HasRaw`); the web rendition is a sibling entry in
`Asset.Renditions`. The HTTP alias `/web` resolves to it.

## Run the tests

```
cd implementation/asset_service
go build ./... && go vet ./... && gofmt -l . && go test ./... -v -race -count=1
```

Real files, real crypto, real `net/http/httptest` servers, race detector on.
See `EVIDENCE.md` for captured verbatim output (34/34 PASS).

---

*Made with love ♥ by Helix Development.*
