package maxadapter

import (
	"context"
	"errors"
	"testing"
)

// TestOneMeClient_LiveCallsAreHonestStubs asserts that every live-network method
// returns ErrNotImplemented. This is the "no bluff" guarantee: the client never
// pretends to have reached a Max server.
func TestOneMeClient_LiveCallsAreHonestStubs(t *testing.T) {
	c := NewOneMeClient(OneMeConfig{})
	ctx := context.Background()

	if err := c.Connect(ctx); !errors.Is(err, ErrNotImplemented) {
		t.Errorf("Connect err = %v, want ErrNotImplemented", err)
	}
	if err := c.Authenticate(ctx, Credentials{Phone: "+70000000000"}); !errors.Is(err, ErrNotImplemented) {
		t.Errorf("Authenticate err = %v, want ErrNotImplemented", err)
	}
	if _, err := c.FetchThreadHistory(ctx, "123"); !errors.Is(err, ErrNotImplemented) {
		t.Errorf("FetchThreadHistory err = %v, want ErrNotImplemented", err)
	}
	if _, err := c.FetchThread(ctx, "123"); !errors.Is(err, ErrNotImplemented) {
		t.Errorf("FetchThread err = %v, want ErrNotImplemented", err)
	}
}

// TestNewOneMeClient_Defaults verifies the documented connection defaults are
// filled in from DefaultOneMeConfig.
func TestNewOneMeClient_Defaults(t *testing.T) {
	c := NewOneMeClient(OneMeConfig{})
	cfg := c.Config()
	if cfg.Endpoint != "wss://ws-api.oneme.ru/websocket" {
		t.Errorf("Endpoint = %q", cfg.Endpoint)
	}
	if cfg.DeviceType != "WEB" {
		t.Errorf("DeviceType = %q", cfg.DeviceType)
	}
	if cfg.ProtocolVer != 11 {
		t.Errorf("ProtocolVer = %d, want 11", cfg.ProtocolVer)
	}
}
