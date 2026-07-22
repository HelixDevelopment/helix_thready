package userservice

import (
	"testing"
	"time"
)

// rfcSecret is the shared secret used by both RFC 4226 Appendix D and RFC 6238
// Appendix B (SHA-1) test vectors: the ASCII string "12345678901234567890".
var rfcSecret = []byte("12345678901234567890")

// TestTOTP_RFC6238KnownVectors validates TOTP codes against the published RFC 6238
// Appendix B test vectors (SHA-1 variant), truncated to 6 digits. The 8-digit RFC
// values reduce to 6 digits by taking them modulo 10^6.
func TestTOTP_RFC6238KnownVectors(t *testing.T) {
	totp := &TOTP{Secret: rfcSecret, Digits: 6, Period: 30, Skew: 0}

	// unixTime -> expected 6-digit code (6-digit truncation of the RFC 8-digit value).
	//   T=59         : RFC 94287082 -> 287082
	//   T=1111111109 : RFC 07081804 -> 081804
	//   T=1111111111 : RFC 14050471 -> 050471
	//   T=1234567890 : RFC 89005924 -> 005924
	//   T=2000000000 : RFC 69279037 -> 279037
	cases := []struct {
		unix int64
		want string
	}{
		{59, "287082"},
		{1111111109, "081804"},
		{1111111111, "050471"},
		{1234567890, "005924"},
		{2000000000, "279037"},
	}
	for _, c := range cases {
		at := time.Unix(c.unix, 0).UTC()
		got := totp.At(at)
		if got != c.want {
			t.Errorf("TOTP.At(T=%d) = %s, want RFC 6238 value %s", c.unix, got, c.want)
		}
		if !totp.Verify(c.want, at) {
			t.Errorf("TOTP.Verify(%s, T=%d) = false, want true", c.want, c.unix)
		}
	}
}

// TestTOTP_RFC4226HOTPVectors validates the HOTP core against the canonical RFC 4226
// Appendix D test values (6 digits, counters 0..9).
func TestTOTP_RFC4226HOTPVectors(t *testing.T) {
	totp := &TOTP{Secret: rfcSecret, Digits: 6, Period: 30}
	want := []string{
		"755224", "287082", "359152", "969429", "338314",
		"254676", "287922", "162583", "399871", "520489",
	}
	for counter, w := range want {
		if got := totp.HOTP(uint64(counter)); got != w {
			t.Errorf("HOTP(counter=%d) = %s, want RFC 4226 value %s", counter, got, w)
		}
	}
}

func TestTOTP_WrongCodeRejected(t *testing.T) {
	totp := &TOTP{Secret: rfcSecret, Digits: 6, Period: 30, Skew: 1}
	at := time.Unix(59, 0)
	if totp.Verify("000000", at) {
		t.Fatal("an incorrect code must be rejected")
	}
	if totp.Verify("287083", at) { // one digit off from the true 287082
		t.Fatal("a near-miss code must be rejected")
	}
}

func TestTOTP_VerifyWindowSkew(t *testing.T) {
	totp := &TOTP{Secret: rfcSecret, Digits: 6, Period: 30, Skew: 1}
	// Code valid for the step centered at T=59 (counter 1) should verify from an instant
	// one step later (counter 2 center) because Skew=1 accepts the previous step.
	code := totp.At(time.Unix(59, 0))
	later := time.Unix(59+30, 0) // next step
	if !totp.Verify(code, later) {
		t.Fatal("code from previous step must verify within skew window")
	}
	// With Skew=0 it must NOT verify two steps away.
	strict := &TOTP{Secret: rfcSecret, Digits: 6, Period: 30, Skew: 0}
	if strict.Verify(code, time.Unix(59+60, 0)) {
		t.Fatal("code must not verify outside the skew window")
	}
}

func TestTOTP_ProvisioningRoundtrip(t *testing.T) {
	totp, b32, err := GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("GenerateTOTPSecret: %v", err)
	}
	if len(totp.Secret) != totpSecretBytes {
		t.Fatalf("secret length = %d, want %d", len(totp.Secret), totpSecretBytes)
	}
	// The base32 provisioning secret must decode back to the same raw bytes.
	back, err := NewTOTPFromBase32(b32)
	if err != nil {
		t.Fatalf("NewTOTPFromBase32: %v", err)
	}
	now := time.Now()
	if totp.At(now) != back.At(now) {
		t.Fatal("base32 provisioning secret did not round-trip to the same code")
	}
	uri := totp.ProvisioningURI("thready-user-service", "alice@example.com")
	if uri == "" || uri[:16] != "otpauth://totp/t" {
		t.Fatalf("unexpected provisioning URI: %q", uri)
	}
}
