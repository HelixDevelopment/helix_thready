package userservice

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Password hashing uses PBKDF2-HMAC-SHA256 from the Go 1.24+ standard library
// (crypto/pbkdf2). Each hash carries its own random salt and its parameters, encoded
// in a self-describing PHC-like string so verification is parameter-agnostic:
//
//	pbkdf2-sha256$<iterations>$<base64-salt>$<base64-derived-key>
//
// Verification is constant-time (crypto/subtle) to avoid timing oracles.
//
// The security model (security-model.md §2) specifies Argon2id; this build uses the
// stdlib PBKDF2 alternative explicitly permitted by the task to keep the module
// dependency-free. The encoding is versioned so an Argon2id scheme can be added later
// without breaking stored hashes.

const (
	// MinPasswordLength is the policy minimum (authn-authz.md §8: "min 12 chars").
	MinPasswordLength = 12

	pbkdf2SaltBytes = 16
	pbkdf2KeyBytes  = 32
	pbkdf2Scheme    = "pbkdf2-sha256"

	// DefaultPBKDF2Iterations is the production cost factor.
	DefaultPBKDF2Iterations = 210_000
)

var (
	// ErrPasswordTooShort is returned by CheckPasswordPolicy for short passwords.
	ErrPasswordTooShort = fmt.Errorf("userservice: password must be at least %d characters", MinPasswordLength)
	// ErrMismatchedHashAndPassword is returned when a password does not match a hash.
	ErrMismatchedHashAndPassword = errors.New("userservice: password does not match hash")
	// ErrInvalidHashFormat is returned when an encoded hash cannot be parsed.
	ErrInvalidHashFormat = errors.New("userservice: invalid password hash format")
)

// Hasher produces and verifies password hashes at a configured cost.
type Hasher struct {
	iterations int
}

// NewHasher returns a Hasher with the given PBKDF2 iteration count. Values below 1
// are clamped to DefaultPBKDF2Iterations.
func NewHasher(iterations int) *Hasher {
	if iterations < 1 {
		iterations = DefaultPBKDF2Iterations
	}
	return &Hasher{iterations: iterations}
}

// DefaultHasher returns a Hasher at the production cost factor.
func DefaultHasher() *Hasher { return NewHasher(DefaultPBKDF2Iterations) }

// CheckPasswordPolicy enforces the minimum-length policy. It does not hash.
func CheckPasswordPolicy(password string) error {
	if utf8.RuneCountInString(password) < MinPasswordLength {
		return ErrPasswordTooShort
	}
	return nil
}

// Hash derives an encoded password hash with a fresh random salt.
func (h *Hasher) Hash(password string) (string, error) {
	salt := make([]byte, pbkdf2SaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("userservice: reading salt: %w", err)
	}
	dk, err := pbkdf2.Key(sha256.New, password, salt, h.iterations, pbkdf2KeyBytes)
	if err != nil {
		return "", fmt.Errorf("userservice: deriving key: %w", err)
	}
	return fmt.Sprintf("%s$%d$%s$%s",
		pbkdf2Scheme,
		h.iterations,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(dk),
	), nil
}

// Verify reports whether password matches the encoded hash. It recomputes the derived
// key using the parameters embedded in the hash and compares in constant time.
func Verify(encoded, password string) error {
	scheme, iterations, salt, want, err := parseEncoded(encoded)
	if err != nil {
		return err
	}
	if scheme != pbkdf2Scheme {
		return ErrInvalidHashFormat
	}
	got, err := pbkdf2.Key(sha256.New, password, salt, iterations, len(want))
	if err != nil {
		return fmt.Errorf("userservice: deriving key: %w", err)
	}
	if subtle.ConstantTimeCompare(got, want) != 1 {
		return ErrMismatchedHashAndPassword
	}
	return nil
}

func parseEncoded(encoded string) (scheme string, iterations int, salt, key []byte, err error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 {
		return "", 0, nil, nil, ErrInvalidHashFormat
	}
	iterations, err = strconv.Atoi(parts[1])
	if err != nil || iterations < 1 {
		return "", 0, nil, nil, ErrInvalidHashFormat
	}
	salt, err = base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil {
		return "", 0, nil, nil, ErrInvalidHashFormat
	}
	key, err = base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil || len(key) == 0 {
		return "", 0, nil, nil, ErrInvalidHashFormat
	}
	return parts[0], iterations, salt, key, nil
}
