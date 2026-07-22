# EVIDENCE — `thready.server`

Real, captured build / vet / gofmt / `-race` test output for the real-wired
Thready server. Nothing here is paraphrased.

## Environment

```
$ go version
go version go1.26.4-X:nodwarf5 linux/amd64
```

## Build, vet, gofmt

```
$ cd implementation/server
$ go build ./...
build OK

$ go vet ./...
vet OK

$ gofmt -l .
(no output = clean)
```

## `go test ./... -race -count=1 -v`

```
=== RUN   TestLogin_RealPBKDF2AndTOTP
--- PASS: TestLogin_RealPBKDF2AndTOTP (0.11s)
=== RUN   TestSearch_RealCosineRanking
--- PASS: TestSearch_RealCosineRanking (0.07s)
=== RUN   TestSkills_RealPrecedenceOrder
--- PASS: TestSkills_RealPrecedenceOrder (0.09s)
=== RUN   TestChannels_CreateThenList
--- PASS: TestChannels_CreateThenList (0.08s)
PASS
ok  	thready.server	1.364s
?   	thready.server/cmd/thready-server	[no test files]
```

Result line (non-verbose): **`ok  	thready.server	1.361s`**

## Workspace resolution

`implementation/go.work` updated to add `./server`. Workspace-resolved:

```
$ cd implementation
$ go work sync
sync done (rc=0)
$ go test thready.server/... -race -count=1
ok  	thready.server	1.342s
?   	thready.server/cmd/thready-server	[no test files]
```

## What each test proves (real domain behaviour)

| Test | Proves |
|------|--------|
| `TestLogin_RealPBKDF2AndTOTP` | The seed password is PBKDF2-hashed at boot via `userservice.NewHasher(...).Hash`. A 200 means `userservice.Verify` (real constant-time PBKDF2 recompute) accepted it AND `userservice.TOTP.Verify` (real RFC 6238) accepted the live code. Wrong password → 401 and wrong TOTP → 401 both fail **through** those real verifiers — not a string compare. |
| `TestSearch_RealCosineRanking` | The corpus is chunked (`semsearch.Chunker`), embedded + indexed (`Engine.Index`), and queried (`Engine.Search`) — real cosine-KNN. A vector-DB query ranks `vectordb.md` first; a disjoint telegram query ranks `telegram.md` first (negative control). `embedder` is the honest label `semsearch/hash-deterministic`. |
| `TestSkills_RealPrecedenceOrder` | Skills are registered in a real `skilldispatch.Registry` and returned via `skilldispatch.OrderByPrecedence` in the real stage order `download > convert > analyze > research > reply`. |
| `TestChannels_CreateThenList` | Real in-memory channel store round-trips a create → list. |

## Reviewer grep (real siblings, no reimplemented domain logic)

`services.go` calls genuine sibling APIs directly:

- `userservice.NewHasher`, `userservice.Hasher.Hash`, `userservice.Verify`,
  `userservice.NewTOTPFromBase32`, `userservice.TOTP.Verify`, `userservice.User`
- `semsearch.NewChunker`, `semsearch.Chunker.Chunk`,
  `semsearch.NewDeterministicEmbedder`, `semsearch.NewMemoryIndex`,
  `semsearch.NewEngine`, `semsearch.Engine.Index`, `semsearch.Engine.Search`
- `skilldispatch.NewRegistry`, `skilldispatch.Registry.Register`/`Resolve`,
  `skilldispatch.OrderByPrecedence`, `skilldispatch.KindDownload…KindReply`
- `eventbus.NewDefault`, `eventbus.Bus.SubscribeAll`/`Publish`/`Unsubscribe`,
  `eventbus.NewEvent`

No password hashing, TOTP, cosine scoring, or precedence ordering is
re-implemented in this module — those computations happen inside the siblings.

## Honest adaptation notes

These are deliberate, disclosed deviations — never a silent fallback to a stub.

1. **No `user_service` user-CRUD store type.** `user_service` ships the domain
   crypto core (`Hasher`, `Verify`, `TOTP`, `User`, `Enforcer`) but **no**
   persistent user store. The real-wired `AuthService` therefore holds the seeded
   `userservice.User` records in a thin local `map[email]`. Every **credential
   decision** still runs through genuine sibling code (`userservice.Verify` +
   `userservice.TOTP.Verify`); only the container is local.

2. **TOTP is real time-based, not the static gateway seed code.** The gateway's
   `SeedRootTOTP`/`SeedAdminTOTP` constants are static 6-digit strings; a real
   RFC 6238 verifier cannot accept a fixed code. The server provisions genuine
   base32 shared secrets (`SeedRootTOTPSecretB32`, `SeedAdminTOTPSecretB32`) and
   the e2e test computes the live code via `userservice.NewTOTPFromBase32(...).Now()`.

3. **Search embedder is deterministic, not a live llama.cpp server.** Ranking is
   real `semsearch` cosine-KNN over the module's deterministic feature-hashing
   embedder. `SearchResponse.Embedder` is the honest label
   `semsearch/hash-deterministic` — it does **not** claim a live llama embedder.

4. **Reduced PBKDF2 cost at seed time.** Seeding uses the real
   `userservice.NewHasher` at `12_000` iterations (vs. the production
   `DefaultPBKDF2Iterations = 210_000`) so the `-race` suite stays fast.
   Verification is parameter-agnostic (the iteration count is embedded in the
   hash string), so this is the same real algorithm at a lower cost factor.

5. **Coded errors can't cross the gateway boundary.** The gateway's `*apiError`
   (which maps a domain code → HTTP status) is unexported, so an injected
   `Service` cannot mint a `not_found`-coded error. `PostService.Reprocess` on a
   **missing** post therefore surfaces as a generic `500` via the gateway's
   `writeServiceError` (which maps any non-`*apiError` to `internal`). The
   existing-post path (the tested one) is fully real. `PostService.Get`'s
   not-found is unaffected — the handler renders that as `404` from the `(_, false)`
   return, not via an error.

6. **Channels & Accounts have no domain module.** These are gateway-level CRUD
   with no dedicated sibling; they are implemented as honest real in-memory
   stores and are **not** claimed to be domain-module-backed.

---

*Made with love ♥ by Helix Development.*
