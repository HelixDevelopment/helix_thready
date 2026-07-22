package threadyconfig

import (
	"errors"
	"strings"
	"testing"
)

func TestLoad_MissingRequiredInProductionAggregates(t *testing.T) {
	// production env with nothing else set: driver defaults to sqlite, JWT alg
	// defaults to HS256, so the required-in-prod set is DB_DSN, JWT_SECRET and
	// ENCRYPTION_KEY.
	_, err := Load(envFunc(map[string]string{"THREADY_ENV": "production"}))
	if err == nil {
		t.Fatal("expected aggregated error for production with no secrets, got nil")
	}
	var me *MultiError
	if !errors.As(err, &me) {
		t.Fatalf("expected *MultiError, got %T", err)
	}
	if len(me.Errors) < 3 {
		t.Errorf("expected at least 3 aggregated errors, got %d: %v", len(me.Errors), me.Errors)
	}
	msg := err.Error()
	for _, want := range []string{"THREADY_DB_DSN", "THREADY_JWT_SECRET", "THREADY_ENCRYPTION_KEY"} {
		if !strings.Contains(msg, want) {
			t.Errorf("aggregated error should name %s; got:\n%s", want, msg)
		}
	}
}

func TestLoad_BadNumericIsNamedError(t *testing.T) {
	_, err := Load(envFunc(map[string]string{
		"THREADY_DB_MAX_OPEN_CONNS": "not-a-number",
	}))
	if err == nil {
		t.Fatal("expected error for bad integer, got nil")
	}
	if !strings.Contains(err.Error(), "THREADY_DB_MAX_OPEN_CONNS") {
		t.Errorf("error should name THREADY_DB_MAX_OPEN_CONNS; got: %v", err)
	}
	if !strings.Contains(err.Error(), "integer") {
		t.Errorf("error should mention integer; got: %v", err)
	}
}

func TestLoad_BadDurationIsNamedError(t *testing.T) {
	_, err := Load(envFunc(map[string]string{
		"THREADY_REQUEST_TIMEOUT": "30 seconds",
	}))
	if err == nil {
		t.Fatal("expected error for bad duration, got nil")
	}
	if !strings.Contains(err.Error(), "THREADY_REQUEST_TIMEOUT") {
		t.Errorf("error should name THREADY_REQUEST_TIMEOUT; got: %v", err)
	}
}

func TestLoad_BadURLIsNamedError(t *testing.T) {
	_, err := Load(envFunc(map[string]string{
		"THREADY_EMBEDDING_BASE_URL": "notaurl",
	}))
	if err == nil {
		t.Fatal("expected error for bad URL, got nil")
	}
	if !strings.Contains(err.Error(), "THREADY_EMBEDDING_BASE_URL") {
		t.Errorf("error should name THREADY_EMBEDDING_BASE_URL; got: %v", err)
	}
}

func TestLoad_BadEnumIsNamedError(t *testing.T) {
	_, err := Load(envFunc(map[string]string{
		"THREADY_DB_DRIVER": "oracle",
	}))
	if err == nil {
		t.Fatal("expected error for bad enum, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "THREADY_DB_DRIVER") {
		t.Errorf("error should name THREADY_DB_DRIVER; got: %v", msg)
	}
	if !strings.Contains(msg, "sqlite") || !strings.Contains(msg, "postgres") {
		t.Errorf("enum error should list allowed values; got: %v", msg)
	}
}

func TestLoad_MultipleBadValuesAllAggregated(t *testing.T) {
	_, err := Load(envFunc(map[string]string{
		"THREADY_RATE_LIMIT_RPS":  "abc",    // bad int
		"THREADY_CACHE_TTL":       "10x",    // bad duration
		"THREADY_NATS_URL":        "://bad", // bad url
		"THREADY_STORAGE_BACKEND": "ftp",    // bad enum
	}))
	if err == nil {
		t.Fatal("expected aggregated error, got nil")
	}
	var me *MultiError
	if !errors.As(err, &me) {
		t.Fatalf("expected *MultiError, got %T", err)
	}
	if len(me.Errors) < 4 {
		t.Errorf("expected at least 4 errors, got %d: %v", len(me.Errors), me.Errors)
	}
	msg := err.Error()
	for _, want := range []string{
		"THREADY_RATE_LIMIT_RPS",
		"THREADY_CACHE_TTL",
		"THREADY_NATS_URL",
		"THREADY_STORAGE_BACKEND",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("aggregated error should name %s; got:\n%s", want, msg)
		}
	}
}

func TestLoad_ConditionalBackendRequirements(t *testing.T) {
	// redis backend without URL -> error naming the URL var.
	_, err := Load(envFunc(map[string]string{
		"THREADY_CACHE_BACKEND": "redis",
	}))
	if err == nil || !strings.Contains(err.Error(), "THREADY_CACHE_REDIS_URL") {
		t.Errorf("expected THREADY_CACHE_REDIS_URL requirement error, got: %v", err)
	}

	// minio backend without endpoint -> error.
	_, err = Load(envFunc(map[string]string{
		"THREADY_STORAGE_BACKEND": "minio",
	}))
	if err == nil || !strings.Contains(err.Error(), "THREADY_STORAGE_ENDPOINT") {
		t.Errorf("expected THREADY_STORAGE_ENDPOINT requirement error, got: %v", err)
	}
}

func TestLoad_ShortJWTSecretRejected(t *testing.T) {
	_, err := Load(envFunc(map[string]string{
		"THREADY_JWT_SIGNING_ALG": "HS256",
		"THREADY_JWT_SECRET":      "tooshort",
	}))
	if err == nil || !strings.Contains(err.Error(), "THREADY_JWT_SECRET") {
		t.Errorf("expected short-secret error naming THREADY_JWT_SECRET, got: %v", err)
	}
}

func TestLoad_ShortEncryptionKeyRejected(t *testing.T) {
	_, err := Load(envFunc(map[string]string{
		"THREADY_ENCRYPTION_KEY": "short-key",
	}))
	if err == nil || !strings.Contains(err.Error(), "THREADY_ENCRYPTION_KEY") {
		t.Errorf("expected short-key error naming THREADY_ENCRYPTION_KEY, got: %v", err)
	}
}

func TestMultiError_UnwrapAndSingle(t *testing.T) {
	sentinel := errors.New("boom")
	me := &MultiError{Errors: []error{sentinel}}
	if !errors.Is(me, sentinel) {
		t.Error("errors.Is should find the wrapped sentinel via Unwrap []error")
	}
	if !strings.Contains(me.Error(), "boom") {
		t.Errorf("single-error message should contain the underlying error, got: %q", me.Error())
	}
}

func TestLoad_NilGetenvUsesDefaults(t *testing.T) {
	c, err := Load(nil)
	if err != nil {
		t.Fatalf("Load(nil) should behave like an empty env, got: %v", err)
	}
	if c.Core.Env != "development" {
		t.Errorf("Load(nil) Env = %q, want development", c.Core.Env)
	}
}
