package threadyconfig

import (
	"fmt"
	"strings"
)

// MultiError aggregates every configuration problem found during Load so the
// caller sees them all at once instead of one-at-a-time. It implements error
// and the Go 1.20 multi-error Unwrap contract.
type MultiError struct {
	Errors []error
}

func (m *MultiError) Error() string {
	switch len(m.Errors) {
	case 0:
		return "configuration: no errors"
	case 1:
		return "configuration error: " + m.Errors[0].Error()
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d configuration errors:", len(m.Errors))
	for _, e := range m.Errors {
		b.WriteString("\n  - ")
		b.WriteString(e.Error())
	}
	return b.String()
}

// Unwrap exposes the aggregated errors to errors.Is/errors.As (Go 1.20+).
func (m *MultiError) Unwrap() []error { return m.Errors }

const minSecretLen = 32

// validate applies required-in-production and conditional cross-field rules,
// appending any problem (naming the offending variable) to p.errs.
func (p *parser) validate(c *Config) {
	isProd := c.Core.Env == "production"

	if isProd {
		if c.Database.DSN == "" {
			p.addf("THREADY_DB_DSN is required in production")
		}
		switch c.Auth.JWTSigningAlg {
		case "HS256":
			if c.Auth.JWTSecret == "" {
				p.addf("THREADY_JWT_SECRET is required in production when THREADY_JWT_SIGNING_ALG=HS256")
			}
		case "RS256", "EdDSA":
			if c.Auth.JWTPrivateKeyPath == "" {
				p.addf("THREADY_JWT_PRIVATE_KEY_PATH is required in production when THREADY_JWT_SIGNING_ALG=%s", c.Auth.JWTSigningAlg)
			}
			if c.Auth.JWTPublicKeyPath == "" {
				p.addf("THREADY_JWT_PUBLIC_KEY_PATH is required in production when THREADY_JWT_SIGNING_ALG=%s", c.Auth.JWTSigningAlg)
			}
		}
		if c.Auth.EncryptionKey == "" {
			p.addf("THREADY_ENCRYPTION_KEY is required in production")
		}
	}

	// Secret strength (any env, only when the secret is actually set).
	if c.Auth.JWTSigningAlg == "HS256" && c.Auth.JWTSecret != "" && len(c.Auth.JWTSecret) < minSecretLen {
		p.addf("THREADY_JWT_SECRET must be at least %d bytes for HS256 (got %d)", minSecretLen, len(c.Auth.JWTSecret))
	}
	if c.Auth.EncryptionKey != "" && len(c.Auth.EncryptionKey) < minSecretLen {
		p.addf("THREADY_ENCRYPTION_KEY must be at least %d bytes (got %d)", minSecretLen, len(c.Auth.EncryptionKey))
	}

	// Numeric sanity.
	if c.Embeddings.Dim <= 0 {
		p.addf("THREADY_EMBEDDING_DIM must be positive (got %d)", c.Embeddings.Dim)
	}
	if c.Auth.PasswordMinLen < 8 {
		p.addf("THREADY_PASSWORD_MIN_LEN must be at least 8 (got %d)", c.Auth.PasswordMinLen)
	}
	if c.Deployment.Remote.PortRangeStart >= c.Deployment.Remote.PortRangeEnd {
		p.addf("CONTAINERS_REMOTE_PORT_RANGE_START (%d) must be less than CONTAINERS_REMOTE_PORT_RANGE_END (%d)",
			c.Deployment.Remote.PortRangeStart, c.Deployment.Remote.PortRangeEnd)
	}

	// Backend-conditional requirements (all environments).
	if c.Database.Driver == "postgres" && c.Database.DSN == "" {
		p.addf("THREADY_DB_DSN is required when THREADY_DB_DRIVER=postgres")
	}
	if c.Cache.Backend == "redis" && c.Cache.RedisURL == "" {
		p.addf("THREADY_CACHE_REDIS_URL is required when THREADY_CACHE_BACKEND=redis")
	}
	if c.EventBus.Backend == "nats" && c.EventBus.NATSURL == "" {
		p.addf("THREADY_NATS_URL is required when THREADY_EVENTBUS_BACKEND=nats")
	}
	if (c.Storage.Backend == "minio" || c.Storage.Backend == "s3") && c.Storage.Endpoint == "" {
		p.addf("THREADY_STORAGE_ENDPOINT is required when THREADY_STORAGE_BACKEND=%s", c.Storage.Backend)
	}
	if c.Vector.Backend == "qdrant" && c.Vector.QdrantURL == "" {
		p.addf("THREADY_QDRANT_URL is required when THREADY_VECTOR_BACKEND=qdrant")
	}
}
