module digital.vasic.threadycli

go 1.26

// The command layer (Run + APIClient interface + DTOs) is stdlib-only and does
// NOT import the SDK. Only the thin real adapter (adapter.go) and the cmd/thready
// entrypoint wrap the sibling sdk_go module, wired here via a filesystem replace
// so the whole thing builds standalone with GOWORK=off (no network, no go.sum).
require digital.vasic.threadysdk v0.0.0

replace digital.vasic.threadysdk => ../sdk_go
