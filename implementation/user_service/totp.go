package userservice

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// TOTP implements RFC 6238 (time-based one-time passwords) over RFC 4226 (HOTP),
// hand-rolled on crypto/hmac + crypto/sha1 per the intended contract. MFA via TOTP is
// mandatory for the Root Admin and Account Admin tiers (authn-authz.md §8).
//
// The default parameters (SHA-1, 6 digits, 30s period, T0=0) are exactly those of the
// RFC 6238 Appendix B / RFC 4226 Appendix D published test vectors, so the
// implementation is verifiable against known-answer data.

const (
	// DefaultTOTPDigits is the standard code length.
	DefaultTOTPDigits = 6
	// DefaultTOTPPeriod is the standard time step in seconds.
	DefaultTOTPPeriod = 30
	// totpSecretBytes is the entropy of a freshly provisioned secret (RFC 4226 §4 recommends >= 128 bits, 160 preferred).
	totpSecretBytes = 20
)

// ErrInvalidBase32Secret is returned when a provisioning secret cannot be decoded.
var ErrInvalidBase32Secret = errors.New("userservice: invalid base32 TOTP secret")

// TOTP holds the parameters for generating and verifying codes for a single secret.
type TOTP struct {
	// Secret is the raw shared secret (not base32).
	Secret []byte
	// Digits is the code length (default 6).
	Digits int
	// Period is the time step in seconds (default 30).
	Period int
	// Skew is the number of adjacent time steps accepted on either side to tolerate
	// clock drift (0 = exact step only).
	Skew int
}

// NewTOTP builds a TOTP from a raw secret with default parameters.
func NewTOTP(secret []byte) *TOTP {
	return &TOTP{Secret: secret, Digits: DefaultTOTPDigits, Period: DefaultTOTPPeriod, Skew: 1}
}

// NewTOTPFromBase32 decodes a base32 provisioning secret (RFC 4648, no padding
// tolerated) and builds a TOTP with default parameters.
func NewTOTPFromBase32(secret string) (*TOTP, error) {
	raw, err := decodeBase32Secret(secret)
	if err != nil {
		return nil, err
	}
	return NewTOTP(raw), nil
}

func (t *TOTP) digits() int {
	if t.Digits <= 0 {
		return DefaultTOTPDigits
	}
	return t.Digits
}

func (t *TOTP) period() int {
	if t.Period <= 0 {
		return DefaultTOTPPeriod
	}
	return t.Period
}

// HOTP computes the RFC 4226 counter-based one-time password.
func (t *TOTP) HOTP(counter uint64) string {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], counter)
	mac := hmac.New(sha1.New, t.Secret)
	mac.Write(buf[:])
	sum := mac.Sum(nil)

	// Dynamic truncation (RFC 4226 §5.3).
	offset := sum[len(sum)-1] & 0x0f
	bin := (uint32(sum[offset]&0x7f) << 24) |
		(uint32(sum[offset+1]) << 16) |
		(uint32(sum[offset+2]) << 8) |
		uint32(sum[offset+3])

	mod := uint32(1)
	for i := 0; i < t.digits(); i++ {
		mod *= 10
	}
	return fmt.Sprintf("%0*d", t.digits(), bin%mod)
}

// counterAt returns the RFC 6238 time counter for an instant (T0 = 0).
func (t *TOTP) counterAt(at time.Time) uint64 {
	return uint64(at.Unix()) / uint64(t.period())
}

// At returns the TOTP code valid at the given instant.
func (t *TOTP) At(at time.Time) string {
	return t.HOTP(t.counterAt(at))
}

// Now returns the TOTP code valid at the current instant.
func (t *TOTP) Now() string { return t.At(time.Now()) }

// Verify reports whether code is valid at instant at, accepting +/- Skew steps. The
// comparison is constant-time to avoid a timing oracle on the code.
func (t *TOTP) Verify(code string, at time.Time) bool {
	code = strings.TrimSpace(code)
	center := t.counterAt(at)
	for d := -t.Skew; d <= t.Skew; d++ {
		c := int64(center) + int64(d)
		if c < 0 {
			continue
		}
		candidate := t.HOTP(uint64(c))
		if subtle.ConstantTimeCompare([]byte(candidate), []byte(code)) == 1 {
			return true
		}
	}
	return false
}

// Base32Secret returns the base32 (RFC 4648, no padding) encoding of the raw secret,
// suitable for provisioning URIs and authenticator apps.
func (t *TOTP) Base32Secret() string {
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(t.Secret)
}

// ProvisioningURI returns an otpauth:// URI for authenticator-app enrolment
// (authn-authz.md §8: enrolment via /v1/auth/mfa/totp/enroll).
func (t *TOTP) ProvisioningURI(issuer, account string) string {
	label := url.PathEscape(issuer + ":" + account)
	q := url.Values{}
	q.Set("secret", t.Base32Secret())
	q.Set("issuer", issuer)
	q.Set("algorithm", "SHA1")
	q.Set("digits", fmt.Sprintf("%d", t.digits()))
	q.Set("period", fmt.Sprintf("%d", t.period()))
	return "otpauth://totp/" + label + "?" + q.Encode()
}

// GenerateTOTPSecret provisions a fresh random secret and returns both the TOTP and its
// base32 provisioning string.
func GenerateTOTPSecret() (*TOTP, string, error) {
	raw := make([]byte, totpSecretBytes)
	if _, err := rand.Read(raw); err != nil {
		return nil, "", err
	}
	t := NewTOTP(raw)
	return t, t.Base32Secret(), nil
}

func decodeBase32Secret(secret string) ([]byte, error) {
	s := strings.ToUpper(strings.TrimSpace(strings.ReplaceAll(secret, " ", "")))
	s = strings.TrimRight(s, "=")
	raw, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(s)
	if err != nil {
		return nil, ErrInvalidBase32Secret
	}
	return raw, nil
}
