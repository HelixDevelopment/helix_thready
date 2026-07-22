# Asset Service — Build & Test Evidence

Physical proof that the `digital.vasic.assetservice` Go module compiles, vets
clean, is gofmt-clean, and passes its full test suite with the race detector
enabled. No mocks for the wire or the crypto: HTTP tests serve real bytes over
`net/http/httptest`, the content store hashes and verifies real files on disk,
and the encrypted store uses real `crypto/aes` + `cipher/gcm` (wrong keys are
rejected by real GCM authentication, not a stub).

- **Gap addressed:** `[BUILD-NEW: Asset Service]` — decoupled store/serve on the
  Catalogizer pattern (architecture:
  `docs/public/research/mvp/architecture/asset-and-download.md` §2, §7, §8).
- **Module path:** `digital.vasic.assetservice`
- **Go directive:** `go 1.26`
- **Go toolchain:** `go version go1.26.4-X:nodwarf5 linux/amd64`
- **Dependencies:** standard library only (no external modules).
- **Date captured:** 2026-07-22

## Command

```
cd implementation/asset_service && go build ./... && go vet ./... && gofmt -l . && go test ./... -v -race -count=1
```

## Captured output (verbatim)

```text
$ go version
go version go1.26.4-X:nodwarf5 linux/amd64

$ go build ./...
(build ok, no output)

$ go vet ./...
(vet ok, no output)

$ gofmt -l .
(gofmt clean, no files listed)

$ go test ./... -v -race -count=1
=== RUN   TestWebRenditionName
--- PASS: TestWebRenditionName (0.00s)
=== RUN   TestAssetHasRawAndRenditionNames
--- PASS: TestAssetHasRawAndRenditionNames (0.00s)
=== RUN   TestContentStorePutIDIsSHA256
--- PASS: TestContentStorePutIDIsSHA256 (0.00s)
=== RUN   TestContentStoreGetRoundTripVerified
--- PASS: TestContentStoreGetRoundTripVerified (0.00s)
=== RUN   TestContentStoreDedup
--- PASS: TestContentStoreDedup (0.00s)
=== RUN   TestContentStoreTamperDetected
--- PASS: TestContentStoreTamperDetected (0.00s)
=== RUN   TestContentStoreGetUnknown
--- PASS: TestContentStoreGetUnknown (0.00s)
=== RUN   TestContentStoreBadID
--- PASS: TestContentStoreBadID (0.00s)
=== RUN   TestContentStoreStat
--- PASS: TestContentStoreStat (0.00s)
=== RUN   TestEncryptedRoundTrip
--- PASS: TestEncryptedRoundTrip (0.00s)
=== RUN   TestEncryptedAtRest
--- PASS: TestEncryptedAtRest (0.00s)
=== RUN   TestEncryptedWrongKeyRejected
--- PASS: TestEncryptedWrongKeyRejected (0.00s)
=== RUN   TestEncryptedDedup
--- PASS: TestEncryptedDedup (0.00s)
=== RUN   TestEncryptedBadKey
--- PASS: TestEncryptedBadKey (0.00s)
=== RUN   TestEncryptedTamperedCiphertextRejected
--- PASS: TestEncryptedTamperedCiphertextRejected (0.00s)
=== RUN   TestHTTPSourceFetchLiveServer
--- PASS: TestHTTPSourceFetchLiveServer (0.00s)
=== RUN   TestHTTPSourceIntoContentStore
--- PASS: TestHTTPSourceIntoContentStore (0.00s)
=== RUN   TestHTTPSourceNotFound
--- PASS: TestHTTPSourceNotFound (0.00s)
=== RUN   TestResolverDenyByDefault
--- PASS: TestResolverDenyByDefault (0.00s)
=== RUN   TestResolverForbidsCrossAccount
--- PASS: TestResolverForbidsCrossAccount (0.00s)
=== RUN   TestResolverAllowsAuthorized
--- PASS: TestResolverAllowsAuthorized (0.00s)
=== RUN   TestResolverUnknownAsset
--- PASS: TestResolverUnknownAsset (0.00s)
=== RUN   TestResolverRendition
--- PASS: TestResolverRendition (0.00s)
=== RUN   TestResolverServesEncryptedStore
--- PASS: TestResolverServesEncryptedStore (0.00s)
=== RUN   TestServeFullContent
--- PASS: TestServeFullContent (0.00s)
=== RUN   TestServeRange
--- PASS: TestServeRange (0.00s)
=== RUN   TestServeForbidden
--- PASS: TestServeForbidden (0.00s)
=== RUN   TestServeNotFound
--- PASS: TestServeNotFound (0.00s)
=== RUN   TestServeRenditionWeb
--- PASS: TestServeRenditionWeb (0.00s)
=== RUN   TestServeNoDirectPath
--- PASS: TestServeNoDirectPath (0.00s)
=== RUN   TestLocalSourceOpenStatSeekable
--- PASS: TestLocalSourceOpenStatSeekable (0.00s)
=== RUN   TestLocalSourceTraversalRefused
--- PASS: TestLocalSourceTraversalRefused (0.00s)
=== RUN   TestLocalSourceNotFound
--- PASS: TestLocalSourceNotFound (0.00s)
=== RUN   TestStubSourcesNotImplemented
--- PASS: TestStubSourcesNotImplemented (0.00s)
PASS
ok  	digital.vasic.assetservice	1.031s
```

## Pass/fail summary

```
ok  digital.vasic.assetservice  1.031s   (34/34 tests PASS, -race, -count=1)
```

Build: OK. Vet: OK. gofmt -l: clean (no files listed). Tests: 34/34 PASS under
the race detector.

## Required-scenario coverage map

| # | Required behavior (from the task) | Test |
|---|-----------------------------------|------|
| 1 | Put a file → id == sha256(content) | `TestContentStorePutIDIsSHA256` |
| 2 | Get → bytes identical + integrity verified | `TestContentStoreGetRoundTripVerified` |
| 3 | Corrupt stored bytes → Get returns integrity error (tamper detection) | `TestContentStoreTamperDetected` |
| 4 | Range read → correct sub-slice via ServeContent (206 + bytes) | `TestServeRange` |
| 5 | Authorizer denies an unauthorized principal (403-equivalent) | `TestServeForbidden`, `TestResolverForbidsCrossAccount`, `TestResolverDenyByDefault` |
| 6 | Authorizer allows an authorized principal | `TestResolverAllowsAuthorized`, `TestServeFullContent` |
| 7 | `…-web` rendition name derived correctly | `TestWebRenditionName`, `TestServeRenditionWeb`, `TestResolverRendition` |
| 8 | AES-GCM encrypted roundtrip works AND wrong key rejected | `TestEncryptedRoundTrip`, `TestEncryptedWrongKeyRejected` |
| 9 | HTTPSource fetches from a live httptest server | `TestHTTPSourceFetchLiveServer`, `TestHTTPSourceIntoContentStore` |

Additional honest-coverage tests: `TestContentStoreDedup` (content-addressed
dedup — one physical blob), `TestEncryptedAtRest` (plaintext never touches
disk), `TestEncryptedTamperedCiphertextRejected` (ciphertext tamper is caught),
`TestEncryptedDedup`, `TestEncryptedBadKey` (non-32-byte key rejected),
`TestResolverServesEncryptedStore` (RBAC + decrypt-only-inside-the-service),
`TestServeNoDirectPath` (no filesystem path leaks to the client),
`TestLocalSourceTraversalRefused` (path-traversal refused),
`TestStubSourcesNotImplemented` (FTP/SMB/NFS/WebDAV honestly fail).

## What is REAL vs. [BUILD-NEW] not-yet-wired

| Capability | Status |
|------------|--------|
| Content-addressed store (sha256), integrity-verify-on-read | **REAL** — `ContentStore` |
| Content-addressed dedup | **REAL** — identical bytes stored once |
| AES-256-GCM encryption at rest, per-asset nonce, wrong-key rejection | **REAL** — `EncryptedStore` (`crypto/aes` + `cipher/gcm`) |
| `LocalSource` (os), `HTTPSource` (net/http remote fetch) | **REAL** |
| HTTP Range / 206 serving via `http.ServeContent` | **REAL** — `Handler` |
| RBAC-gated resolution, deny-by-default, never-a-direct-path | **REAL** — `Resolver` + `Authorizer` |
| `…-web` rendition-name derivation | **REAL** — `WebRenditionName` |
| FTP / SMB / NFS / WebDAV `FileSource` | **[BUILD-NEW] not-yet-wired** — `NewStubSource` returns `ErrNotImplemented`; documented reuse seam for `digital.vasic.filesystem`, never faked |
| HLS/DASH manifests, transcoder (H.264/AAC/AV1) | **out of scope for the core** — `[OPEN: ASSET-2]`; not claimed here |
| Persistent relational SoR for the asset index | **out of scope for the core** — `AssetIndex` is in-memory; the encrypted-store plaintext→ciphertext map IS durable on disk |

## Reproduce

```
cd implementation/asset_service
go build ./... && go vet ./... && gofmt -l . && go test ./... -v -race -count=1
```

## Verdict

**READY.** The Asset Service core is real and fully exercised under `-race`:
content-addressed storage with integrity-verify-on-read and tamper detection,
AES-256-GCM encryption at rest with genuine wrong-key rejection, RBAC-gated
resolution that never exposes a filesystem path, HTTP Range/206 serving via
`http.ServeContent`, a live-server `HTTPSource`, and the `…-web` rendition
convention. FTP/SMB/NFS/WebDAV are honest `ErrNotImplemented` stubs — the
`digital.vasic.filesystem` reuse seam, `[BUILD-NEW]`, not yet wired. HLS/DASH
transcoding is explicitly out of the core's scope (`[OPEN: ASSET-2]`) and is not
claimed to work.
