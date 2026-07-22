package maxadapter

import "context"

// OneMeConfig holds the connection parameters for the internal OneMe WebSocket
// API. Defaults match what the reference clients (vkmax, PyMax) send.
type OneMeConfig struct {
	// Endpoint is the OneMe WebSocket URL. CONFIRMED from vkmax
	// (client.py: WS_HOST) and MaxAPI docs.
	Endpoint string
	// DeviceID is echoed in the handshake (opcode 6) and reused for token login.
	DeviceID string
	// UserAgent.deviceType, e.g. "WEB", sent in the handshake payload.
	DeviceType string
	// ProtocolVer is the envelope "ver" field (11 in all observed captures).
	ProtocolVer int
}

// DefaultOneMeConfig returns the connection defaults used by the reference
// clients. These values are documentary; no socket is opened here.
func DefaultOneMeConfig() OneMeConfig {
	return OneMeConfig{
		Endpoint:    "wss://ws-api.oneme.ru/websocket",
		DeviceType:  "WEB",
		ProtocolVer: 11,
	}
}

// OneMeClient is the [BUILD-NEW] live client for the internal OneMe WebSocket
// API. It is a compile-time-complete implementation of MaxClient whose network
// methods are honest stubs: each returns ErrNotImplemented because this
// environment has no Max account and the on-wire session is unverified. The
// real, tested capability of this package is ParseHistory, which these stubs
// are wired to call once a live transport exists.
//
// It also satisfies the threadreader.MessageSource shape via FetchThread, so
// wiring it into the assembler is a one-liner once the transport lands.
type OneMeClient struct {
	cfg   OneMeConfig
	creds Credentials
}

// NewOneMeClient constructs a client with the given config. A zero-value config
// field falls back to DefaultOneMeConfig.
func NewOneMeClient(cfg OneMeConfig) *OneMeClient {
	def := DefaultOneMeConfig()
	if cfg.Endpoint == "" {
		cfg.Endpoint = def.Endpoint
	}
	if cfg.DeviceType == "" {
		cfg.DeviceType = def.DeviceType
	}
	if cfg.ProtocolVer == 0 {
		cfg.ProtocolVer = def.ProtocolVer
	}
	return &OneMeClient{cfg: cfg}
}

// Config returns the effective connection config (useful for tests/inspection).
func (c *OneMeClient) Config() OneMeConfig { return c.cfg }

// Connect would open the WebSocket and perform the opcode-6 handshake.
//
// [BUILD-NEW]: not implemented. Opening a real socket and completing the
// handshake needs on-wire confirmation this environment cannot provide, so this
// fails loudly rather than fake a connection.
func (c *OneMeClient) Connect(ctx context.Context) error {
	return ErrNotImplemented
}

// Authenticate would run the phone SMS flow (opcode 17 -> 18) or a token login
// (opcode 19) and retain the resulting session token.
//
// [BUILD-NEW]: not implemented. The credentials are retained so a future real
// implementation has them, but no auth traffic is sent.
func (c *OneMeClient) Authenticate(ctx context.Context, creds Credentials) error {
	c.creds = creds
	return ErrNotImplemented
}

// FetchThreadHistory would send an opcode-49 history request for threadID and
// pass the response body to ParseHistory.
//
// [BUILD-NEW]: the NETWORK half is not implemented and returns
// ErrNotImplemented. The MAPPING half it would call — ParseHistory — is real
// and independently tested; see parse_test.go.
func (c *OneMeClient) FetchThreadHistory(ctx context.Context, threadID string) ([]Post, error) {
	// Intended shape once a transport exists:
	//   frame := buildFrame(49, map[string]any{
	//       "chatId": threadID, "from": nowMillis(),
	//       "forward": 0, "backward": 40, "getMessages": true,
	//   })
	//   resp, err := c.invoke(ctx, frame) // <- [BUILD-NEW] live send/recv
	//   if err != nil { return nil, err }
	//   return ParseHistory(resp)
	return nil, ErrNotImplemented
}

// FetchThread adapts FetchThreadHistory to the threadreader.MessageSource
// signature (FetchThread(ctx, threadID) ([]Post, error)). This is the exact
// seam the assembler consumes.
func (c *OneMeClient) FetchThread(ctx context.Context, threadID string) ([]Post, error) {
	return c.FetchThreadHistory(ctx, threadID)
}

// compile-time assertion: OneMeClient implements MaxClient.
var _ MaxClient = (*OneMeClient)(nil)
