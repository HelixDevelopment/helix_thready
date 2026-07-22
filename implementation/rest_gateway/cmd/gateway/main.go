// Command gateway runs the Helix Thready REST /v1 gateway with the seeded
// in-memory services. It is a real, runnable HTTP server; the backing services
// are honest in-memory stubs pending the go.work integration with the sibling
// implementation/* domain modules.
package main

import (
	"crypto/rand"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	gateway "digital.vasic.restgateway"
)

func main() {
	addr := os.Getenv("GATEWAY_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		log.Fatalf("gateway: failed to generate signing secret: %v", err)
	}
	signer, err := gateway.NewSigner(gateway.SignerConfig{Secret: secret})
	if err != nil {
		log.Fatalf("gateway: signer: %v", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	srv := gateway.New(gateway.NewInMemoryServices(), signer, gateway.WithLogger(logger))

	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           srv,
		ReadHeaderTimeout: 10 * time.Second,
	}

	logger.Info("rest-gateway listening", "addr", addr)
	if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("gateway: server error: %v", err)
	}
}
