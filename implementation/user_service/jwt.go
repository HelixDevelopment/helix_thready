package userservice

import (
	"crypto"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// JWT support is hand-rolled over the standard library (crypto/hmac, crypto/rsa,
// encoding/base64, encoding/json) per the intended contract (authn-authz.md §3).
//
// HS256 is the default (matches the module's jwt.DefaultConfig). RS256 is supported
// for the multi-service verification story ([GAP: #10 / 7.2]) where every service must
// verify with a public key without holding the signing secret.
//
// The critical negative control from the contract is enforced: Validate PINS the
// algorithm to the configured one, so "alg: none" and HS256-signed-with-the-RSA-public-
// key downgrade attacks are rejected.

// Algorithm is a supported JWT signing algorithm.
type Algorithm string

const (
	// HS256 is HMAC-SHA256 (symmetric); the default.
	HS256 Algorithm = "HS256"
	// RS256 is RSASSA-PKCS1-v1_5 with SHA-256 (asymmetric).
	RS256 Algorithm = "RS256"
)

// Token type values carried in the token_type claim.
const (
	tokenTypeAccess  = "access"
	tokenTypeRefresh = "refresh"
)

var (
	// ErrInvalidToken is returned for structurally invalid or malformed tokens.
	ErrInvalidToken = errors.New("userservice: invalid token")
	// ErrSignatureInvalid is returned when the signature does not verify.
	ErrSignatureInvalid = errors.New("userservice: token signature invalid")
	// ErrAlgorithmMismatch is returned when the token's alg differs from the configured
	// algorithm (algorithm-confusion / downgrade protection).
	ErrAlgorithmMismatch = errors.New("userservice: token algorithm mismatch")
	// ErrTokenExpired is returned when exp is in the past.
	ErrTokenExpired = errors.New("userservice: token expired")
	// ErrTokenNotYetValid is returned when nbf is in the future.
	ErrTokenNotYetValid = errors.New("userservice: token not yet valid")
	// ErrTokenRevoked is returned when the token's jti is revoked (or unknown) in the store.
	ErrTokenRevoked = errors.New("userservice: token revoked")
	// ErrWrongTokenType is returned when a refresh operation gets a non-refresh token.
	ErrWrongTokenType = errors.New("userservice: wrong token type")
	// ErrIssuerMismatch / ErrAudienceMismatch guard the standard registered claims.
	ErrIssuerMismatch   = errors.New("userservice: issuer mismatch")
	ErrAudienceMismatch = errors.New("userservice: audience mismatch")
)

// Claims is the JWT payload used by the User Service (authn-authz.md §3).
type Claims struct {
	Issuer    string   `json:"iss,omitempty"`
	Subject   string   `json:"sub,omitempty"`
	Audience  string   `json:"aud,omitempty"`
	ExpiresAt int64    `json:"exp,omitempty"`
	IssuedAt  int64    `json:"iat,omitempty"`
	NotBefore int64    `json:"nbf,omitempty"`
	JTI       string   `json:"jti,omitempty"`
	Role      string   `json:"role,omitempty"`
	AccountID string   `json:"account_id,omitempty"`
	Scopes    []string `json:"scopes,omitempty"`
	TokenType string   `json:"token_type,omitempty"`
}

type jwtHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
	Kid string `json:"kid,omitempty"`
}

// Config configures a JWT Manager.
type Config struct {
	Algorithm  Algorithm
	Issuer     string
	Audience   string
	AccessTTL  time.Duration
	RefreshTTL time.Duration
	KeyID      string

	// HMACSecret is required for HS256.
	HMACSecret []byte
	// RSAPrivate signs RS256 tokens; RSAPublic verifies them. If RSAPublic is nil it
	// defaults to RSAPrivate.Public().
	RSAPrivate *rsa.PrivateKey
	RSAPublic  *rsa.PublicKey

	// Now is an injectable clock (defaults to time.Now).
	Now func() time.Time
}

// TokenPair is the access + refresh pair returned to a client (authn-authz.md §3).
type TokenPair struct {
	AccessToken  string
	RefreshToken string
	TokenType    string // always "Bearer"
	ExpiresIn    int64  // access-token lifetime in seconds
}

// Manager issues and validates JWTs and enforces revocation via a TokenStore.
type Manager struct {
	cfg   Config
	store TokenStore
}

// NewManager validates the config and returns a Manager. The store is required for
// revocation semantics (refresh rotation and logout-all).
func NewManager(cfg Config, store TokenStore) (*Manager, error) {
	if store == nil {
		return nil, errors.New("userservice: token store is required")
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.AccessTTL <= 0 {
		cfg.AccessTTL = 15 * time.Minute
	}
	if cfg.RefreshTTL <= 0 {
		cfg.RefreshTTL = 7 * 24 * time.Hour
	}
	switch cfg.Algorithm {
	case HS256:
		if len(cfg.HMACSecret) == 0 {
			return nil, errors.New("userservice: HS256 requires HMACSecret")
		}
	case RS256:
		if cfg.RSAPrivate == nil {
			return nil, errors.New("userservice: RS256 requires RSAPrivate")
		}
		if cfg.RSAPublic == nil {
			cfg.RSAPublic = &cfg.RSAPrivate.PublicKey
		}
	default:
		return nil, fmt.Errorf("userservice: unsupported algorithm %q", cfg.Algorithm)
	}
	return &Manager{cfg: cfg, store: store}, nil
}

// Issue mints an access + refresh pair for a subject, registering both jtis in the
// store so they can be revoked.
func (m *Manager) Issue(subject, role, accountID string, scopes []string) (TokenPair, error) {
	now := m.cfg.Now()

	access := Claims{
		Issuer:    m.cfg.Issuer,
		Subject:   subject,
		Audience:  m.cfg.Audience,
		ExpiresAt: now.Add(m.cfg.AccessTTL).Unix(),
		IssuedAt:  now.Unix(),
		NotBefore: now.Unix(),
		JTI:       newJTI(),
		Role:      role,
		AccountID: accountID,
		Scopes:    scopes,
		TokenType: tokenTypeAccess,
	}
	refresh := Claims{
		Issuer:    m.cfg.Issuer,
		Subject:   subject,
		Audience:  m.cfg.Audience,
		ExpiresAt: now.Add(m.cfg.RefreshTTL).Unix(),
		IssuedAt:  now.Unix(),
		NotBefore: now.Unix(),
		JTI:       newJTI(),
		Role:      role,
		AccountID: accountID,
		Scopes:    scopes,
		TokenType: tokenTypeRefresh,
	}

	at, err := m.sign(access)
	if err != nil {
		return TokenPair{}, err
	}
	rt, err := m.sign(refresh)
	if err != nil {
		return TokenPair{}, err
	}

	m.store.Register(access.JTI, subject, time.Unix(access.ExpiresAt, 0))
	m.store.Register(refresh.JTI, subject, time.Unix(refresh.ExpiresAt, 0))

	return TokenPair{
		AccessToken:  at,
		RefreshToken: rt,
		TokenType:    "Bearer",
		ExpiresIn:    int64(m.cfg.AccessTTL.Seconds()),
	}, nil
}

// Validate parses and cryptographically verifies a token, checks the standard claims
// (exp/nbf/iss/aud), pins the algorithm, and rejects revoked/unknown jtis.
func (m *Manager) Validate(token string) (*Claims, error) {
	claims, err := m.verify(token)
	if err != nil {
		return nil, err
	}
	now := m.cfg.Now()
	if claims.ExpiresAt != 0 && now.Unix() >= claims.ExpiresAt {
		return nil, ErrTokenExpired
	}
	if claims.NotBefore != 0 && now.Unix() < claims.NotBefore {
		return nil, ErrTokenNotYetValid
	}
	if m.cfg.Issuer != "" && claims.Issuer != m.cfg.Issuer {
		return nil, ErrIssuerMismatch
	}
	if m.cfg.Audience != "" && claims.Audience != m.cfg.Audience {
		return nil, ErrAudienceMismatch
	}
	if claims.JTI != "" && !m.store.Active(claims.JTI) {
		return nil, ErrTokenRevoked
	}
	return claims, nil
}

// Refresh validates a refresh token, atomically revokes it (rotation), and issues a fresh
// pair. The old refresh token can never be used again (authn-authz.md §3).
//
// Rotation is guarded by a single atomic compare-and-revoke (store.RevokeIfActive), NOT by
// the Active read inside Validate: Validate verifies the signature, expiry and claims, but
// its Active check and a subsequent Revoke would be two separate steps, so concurrent
// presentations of the same refresh token could all pass Validate before any Revoke landed
// and every one would receive a new pair (refresh-token replay / double issuance). Making
// active→revoked one indivisible transition means exactly one caller wins and may Issue;
// all others — a lost race or a genuine replay — lose the CAS and are rejected. Expiry
// remains authoritatively the signed exp claim checked in Validate (single source of truth).
func (m *Manager) Refresh(refreshToken string) (TokenPair, error) {
	claims, err := m.Validate(refreshToken)
	if err != nil {
		return TokenPair{}, err
	}
	if claims.TokenType != tokenTypeRefresh {
		return TokenPair{}, ErrWrongTokenType
	}
	// Single atomic gate: only the caller that transitions this jti active→revoked may
	// mint the replacement. Losers (replay or a concurrent duplicate) get ErrTokenRevoked.
	if !m.store.RevokeIfActive(claims.JTI) {
		return TokenPair{}, ErrTokenRevoked
	}
	return m.Issue(claims.Subject, claims.Role, claims.AccountID, claims.Scopes)
}

// Revoke invalidates a single token by its jti (used for logout of one session).
func (m *Manager) Revoke(token string) error {
	claims, err := m.verify(token)
	if err != nil {
		return err
	}
	if claims.JTI == "" || !m.store.Revoke(claims.JTI) {
		return ErrTokenRevoked
	}
	return nil
}

// RevokeAllForUser revokes every active token for a subject (logout-all).
func (m *Manager) RevokeAllForUser(userID string) int {
	return m.store.RevokeAllForUser(userID)
}

// --- signing / verification internals ---

func (m *Manager) sign(claims Claims) (string, error) {
	header := jwtHeader{Alg: string(m.cfg.Algorithm), Typ: "JWT", Kid: m.cfg.KeyID}
	hb, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	pb, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	signingInput := b64(hb) + "." + b64(pb)

	sig, err := m.computeSignature([]byte(signingInput))
	if err != nil {
		return "", err
	}
	return signingInput + "." + b64(sig), nil
}

func (m *Manager) verify(token string) (*Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrInvalidToken
	}
	hb, err := unb64(parts[0])
	if err != nil {
		return nil, ErrInvalidToken
	}
	var header jwtHeader
	if err := json.Unmarshal(hb, &header); err != nil {
		return nil, ErrInvalidToken
	}
	// Algorithm-confusion / downgrade protection: pin to the configured algorithm.
	// This rejects "alg: none" and HS256-signed-with-RSA-public-key attacks.
	if header.Alg != string(m.cfg.Algorithm) {
		return nil, ErrAlgorithmMismatch
	}
	sig, err := unb64(parts[2])
	if err != nil {
		return nil, ErrInvalidToken
	}
	signingInput := []byte(parts[0] + "." + parts[1])
	if err := m.verifySignature(signingInput, sig); err != nil {
		return nil, err
	}
	pb, err := unb64(parts[1])
	if err != nil {
		return nil, ErrInvalidToken
	}
	var claims Claims
	if err := json.Unmarshal(pb, &claims); err != nil {
		return nil, ErrInvalidToken
	}
	return &claims, nil
}

func (m *Manager) computeSignature(signingInput []byte) ([]byte, error) {
	switch m.cfg.Algorithm {
	case HS256:
		mac := hmac.New(sha256.New, m.cfg.HMACSecret)
		mac.Write(signingInput)
		return mac.Sum(nil), nil
	case RS256:
		digest := sha256.Sum256(signingInput)
		return rsa.SignPKCS1v15(rand.Reader, m.cfg.RSAPrivate, crypto.SHA256, digest[:])
	default:
		return nil, fmt.Errorf("userservice: unsupported algorithm %q", m.cfg.Algorithm)
	}
}

func (m *Manager) verifySignature(signingInput, sig []byte) error {
	switch m.cfg.Algorithm {
	case HS256:
		mac := hmac.New(sha256.New, m.cfg.HMACSecret)
		mac.Write(signingInput)
		if subtle.ConstantTimeCompare(sig, mac.Sum(nil)) != 1 {
			return ErrSignatureInvalid
		}
		return nil
	case RS256:
		digest := sha256.Sum256(signingInput)
		if err := rsa.VerifyPKCS1v15(m.cfg.RSAPublic, crypto.SHA256, digest[:], sig); err != nil {
			return ErrSignatureInvalid
		}
		return nil
	default:
		return fmt.Errorf("userservice: unsupported algorithm %q", m.cfg.Algorithm)
	}
}

func b64(b []byte) string            { return base64.RawURLEncoding.EncodeToString(b) }
func unb64(s string) ([]byte, error) { return base64.RawURLEncoding.DecodeString(s) }

// newJTI returns a random, unique token identifier. It combines 16 random bytes with a
// monotonic-ish nanosecond suffix so two tokens minted in the same call are distinct.
func newJTI() string {
	var buf [16]byte
	_, _ = rand.Read(buf[:])
	var ns [8]byte
	binary.BigEndian.PutUint64(ns[:], uint64(time.Now().UnixNano()))
	return hex.EncodeToString(buf[:]) + hex.EncodeToString(ns[:])
}
