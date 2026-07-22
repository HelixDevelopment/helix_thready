package server

import (
	"fmt"
	"net/http"

	eventbus "digital.vasic.eventbusservice"
	gateway "digital.vasic.restgateway"
)

// jwtSecret is the symmetric HS256 secret the gateway signs/verifies access
// tokens with. It is a fixed dev secret so the standalone binary is
// self-consistent; a production deployment would inject it from config/secrets.
var jwtSecret = []byte("thready-server-dev-secret-please-rotate-01")

// NewServer assembles the REAL-wired gateway.Services (auth -> user_service,
// search -> semantic_search, skills/posts -> skill_dispatch, events ->
// event_bus_service, channels/accounts -> honest in-memory stores) and returns
// the composed gateway HTTP handler.
func NewServer() (http.Handler, error) {
	auth, err := newRealAuth()
	if err != nil {
		return nil, fmt.Errorf("server: wiring auth: %w", err)
	}
	search, err := newRealSearch()
	if err != nil {
		return nil, fmt.Errorf("server: wiring search: %w", err)
	}

	registry, skills := buildSkills()

	svc := gateway.Services{
		Auth:     auth,
		Channels: newRealChannels(),
		Posts:    newRealPosts(registry),
		Search:   search,
		Skills:   &realSkills{skills: skills},
		Accounts: newRealAccounts(),
		Events:   &realEvents{bus: eventbus.NewDefault()},
	}

	signer, err := gateway.NewSigner(gateway.SignerConfig{Secret: jwtSecret})
	if err != nil {
		return nil, fmt.Errorf("server: building signer: %w", err)
	}

	return gateway.New(svc, signer), nil
}
