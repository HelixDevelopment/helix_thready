package gateway

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// This file is a tiny, self-contained JWT verifier/signer built on
// crypto/hmac + encoding/base64 (stdlib only). It implements HS256 with an
// algorithm pin on verify (rejecting alg:none / algorithm-confusion), mirroring
// the guarantee documented in api/authn-authz.md §3. The production system
// signs RS256/EdDSA via JWKS; here HMAC keeps the gateway self-testable without
// external key material. The claim shape matches the documented access-token
// claims (sub/role/account_id/scopes/iss/aud/iat/exp).

// Claims is the JWT payload.
type Claims struct {
	Sub       string   `json:"sub"`
	Role      string   `json:"role"`
	AccountID string   `json:"account_id,omitempty"`
	Scopes    []string `json:"scopes"`
	Iss       string   `json:"iss,omitempty"`
	Aud       string   `json:"aud,omitempty"`
	Iat       int64    `json:"iat"`
	Exp       int64    `json:"exp"`
	TokenType string   `json:"token_use,omitempty"` // "access" | "refresh"
}

// jwtHeader is the fixed HS256 header.
type jwtHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
	Kid string `json:"kid,omitempty"`
}

// Signer signs and verifies HS256 JWTs with a symmetric secret.
type Signer struct {
	secret   []byte
	issuer   string
	audience string
	kid      string
	now      func() time.Time
}

// SignerConfig configures a Signer.
type SignerConfig struct {
	Secret   []byte
	Issuer   string
	Audience string
	Kid      string
}

// NewSigner builds a Signer. A zero-length secret is rejected.
func NewSigner(cfg SignerConfig) (*Signer, error) {
	if len(cfg.Secret) == 0 {
		return nil, errors.New("signer secret must be non-empty")
	}
	iss := cfg.Issuer
	if iss == "" {
		iss = "thready-user-service"
	}
	aud := cfg.Audience
	if aud == "" {
		aud = "thready-api"
	}
	kid := cfg.Kid
	if kid == "" {
		kid = "2026-07-a1b2c3d4"
	}
	return &Signer{secret: cfg.Secret, issuer: iss, audience: aud, kid: kid, now: time.Now}, nil
}

var errInvalidToken = errors.New("invalid token")

// Sign returns a signed compact JWT for the given claims and TTL. iss/aud/iat/exp
// are stamped from the signer config.
func (s *Signer) Sign(c Claims, ttl time.Duration) (string, error) {
	now := s.now()
	c.Iss = s.issuer
	c.Aud = s.audience
	c.Iat = now.Unix()
	c.Exp = now.Add(ttl).Unix()

	headerJSON, err := json.Marshal(jwtHeader{Alg: "HS256", Typ: "JWT", Kid: s.kid})
	if err != nil {
		return "", err
	}
	payloadJSON, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	seg := b64(headerJSON) + "." + b64(payloadJSON)
	sig := s.mac([]byte(seg))
	return seg + "." + b64(sig), nil
}

// Verify parses and validates a compact JWT: it pins the algorithm to HS256,
// verifies the HMAC in constant time, and checks expiry. On any failure it
// returns errInvalidToken (callers map this to 401 unauthenticated).
func (s *Signer) Verify(token string) (*Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errInvalidToken
	}
	headerRaw, err := unb64(parts[0])
	if err != nil {
		return nil, errInvalidToken
	}
	var h jwtHeader
	if err := json.Unmarshal(headerRaw, &h); err != nil {
		return nil, errInvalidToken
	}
	// Algorithm pin: reject alg:none and any non-HS256 (algorithm-confusion).
	if h.Alg != "HS256" {
		return nil, errInvalidToken
	}
	expected := s.mac([]byte(parts[0] + "." + parts[1]))
	got, err := unb64(parts[2])
	if err != nil {
		return nil, errInvalidToken
	}
	if !hmac.Equal(expected, got) {
		return nil, errInvalidToken
	}
	payloadRaw, err := unb64(parts[1])
	if err != nil {
		return nil, errInvalidToken
	}
	var c Claims
	if err := json.Unmarshal(payloadRaw, &c); err != nil {
		return nil, errInvalidToken
	}
	if c.Exp != 0 && s.now().Unix() >= c.Exp {
		return nil, errInvalidToken
	}
	return &c, nil
}

func (s *Signer) mac(msg []byte) []byte {
	m := hmac.New(sha256.New, s.secret)
	m.Write(msg)
	return m.Sum(nil)
}

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func unb64(s string) ([]byte, error) { return base64.RawURLEncoding.DecodeString(s) }
