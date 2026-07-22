package server

import (
	"fmt"
	"net/http"
	"os"

	eventbus "digital.vasic.eventbusservice"
	gateway "digital.vasic.restgateway"
)

// jwtSecretEnv names the environment variable the HS256 signing secret is loaded
// from at runtime. Per constitution §11.4.10 secrets are runtime-load-only from
// the environment — never hardcoded or committed — so NewServer fails closed
// when it is unset (no in-code fallback that would let anyone forge tokens).
const jwtSecretEnv = "THREADY_JWT_SECRET"

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

	// Fail closed: the signing secret MUST be provided at runtime via the
	// environment. An unset/empty value is a hard error — never a hardcoded
	// fallback, which would let anyone running the binary forge tokens.
	secret := os.Getenv(jwtSecretEnv)
	if secret == "" {
		return nil, fmt.Errorf("server: %s is required (refusing to start with no signing secret)", jwtSecretEnv)
	}

	signer, err := gateway.NewSigner(gateway.SignerConfig{Secret: []byte(secret)})
	if err != nil {
		return nil, fmt.Errorf("server: building signer: %w", err)
	}

	return gateway.New(svc, signer), nil
}
