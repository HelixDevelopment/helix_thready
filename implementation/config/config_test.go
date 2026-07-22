package threadyconfig

import (
	"strings"
	"testing"
	"time"
)

// envFunc returns a getenv closure backed by m.
func envFunc(m map[string]string) Getenv {
	return func(k string) string { return m[k] }
}

// fullEnv is a complete, valid production-grade environment exercising every
// typed field kind (enum, int, float, bool, duration, url) plus secrets.
func fullEnv() map[string]string {
	return map[string]string{
		"THREADY_ENV":              "production",
		"THREADY_ENV_FILE":         "/etc/thready/prod.env",
		"THREADY_LOG_LEVEL":        "debug",
		"THREADY_LOG_FORMAT":       "json",
		"THREADY_PUBLIC_DOMAIN":    "thready.hxd3v.com",
		"THREADY_PUBLIC_BASE_URL":  "https://thready.hxd3v.com",
		"THREADY_HTTP_ADDR":        "0.0.0.0:9443",
		"THREADY_HTTP3_ENABLED":    "false",
		"THREADY_HTTP_COMPRESSION": "gzip",
		"THREADY_REQUEST_TIMEOUT":  "45s",
		"THREADY_RATE_LIMIT_RPS":   "250",
		"THREADY_CORS_ORIGINS":     "https://app.example.com",
		"THREADY_TLS_MIN_VERSION":  "1.2",
		"LETS_ENCRYPT_EMAIL":       "ops@hxd3v.com",
		"LETS_ENCRYPT_CHALLENGE":   "dns-01",

		"CONTAINERS_REMOTE_ENABLED":             "true",
		"CONTAINERS_REMOTE_DEFAULT_RUNTIME":     "docker",
		"CONTAINERS_REMOTE_PORT_RANGE_START":    "21000",
		"CONTAINERS_REMOTE_PORT_RANGE_END":      "22000",
		"CONTAINERS_REMOTE_CONNECT_TIMEOUT":     "15s",
		"CONTAINERS_REMOTE_SSH_MAX_CONNECTIONS": "5",
		"CONTAINERS_REMOTE_SCHEDULER":           "leastloaded",

		"THREADY_DB_DRIVER":            "postgres",
		"THREADY_DB_DSN":               "postgres://thready:secretpw@db:5432/thready?sslmode=require",
		"THREADY_DB_MAX_OPEN_CONNS":    "64",
		"THREADY_DB_MAX_IDLE_CONNS":    "16",
		"THREADY_DB_CONN_MAX_LIFETIME": "1h",
		"THREADY_DB_MIGRATE_ON_BOOT":   "false",
		"THREADY_DB_PARTITIONING":      "true",

		"THREADY_VECTOR_BACKEND": "qdrant",
		"THREADY_VECTOR_DSN":     "postgres://thready:secretpw@db:5432/thready",
		"THREADY_VECTOR_METRIC":  "dot",
		"THREADY_VECTOR_INDEX":   "ivfflat",
		"THREADY_QDRANT_URL":     "http://qdrant:6333",
		"THREADY_EMBEDDING_DIM":  "768",

		"HELIX_EMBEDDING_PROVIDER":   "llama",
		"THREADY_EMBEDDING_BASE_URL": "http://llm:8080/v1",
		"THREADY_EMBEDDING_MODEL":    "jina-embeddings-v2-base-code",
		"THREADY_EMBEDDING_API_KEY":  "emb-secret-key",

		"HELIX_LLM_BASE_URL":          "http://llm:8080",
		"HELIX_LLM_MODEL":             "Llama-3.1-70B-Instruct-Q4_K_M",
		"HELIX_LLM_CODE_MODEL":        "Qwen2.5-Coder",
		"THREADY_LLM_MAX_RETRIES":     "7",
		"THREADY_LLM_CIRCUIT_BREAKER": "false",
		"OPENAI_API_KEY":              "sk-openai-secret",
		"ANTHROPIC_API_KEY":           "sk-ant-secret",
		"REPLICATE_API_TOKEN":         "r8-replicate-secret",

		"HELIX_VISION_PROVIDER":       "ollama",
		"HELIX_VISION_TIMEOUT":        "90",
		"HELIX_VISION_MAX_IMAGE_SIZE": "2048",
		"HELIX_VISION_OPENCV_ENABLED": "false",
		"HELIX_VISION_SSIM_THRESHOLD": "0.88",
		"HELIX_OLLAMA_URL":            "http://ollama:11434",
		"HELIX_OLLAMA_MODEL":          "minicpm-v:8b",
		"ASTICA_API_KEY":              "astica-secret",

		"THREADY_OCR_PROVIDER": "tesseract",
		"THREADY_OCR_LANGS":    "eng,rus,srp",

		"THREADY_CACHE_BACKEND":   "redis",
		"THREADY_CACHE_REDIS_URL": "redis://cache:6379/0",
		"THREADY_CACHE_TTL":       "20m",

		"THREADY_STORAGE_BACKEND":        "minio",
		"THREADY_STORAGE_ENDPOINT":       "https://minio:9000",
		"THREADY_STORAGE_BUCKET":         "thready-prod",
		"THREADY_STORAGE_ACCESS_KEY":     "minio-access",
		"THREADY_STORAGE_SECRET_KEY":     "minio-secret",
		"THREADY_STORAGE_SIGNED_URL_TTL": "5m",
		"THREADY_MEDIA_DIR":              "/data/media",
		"THREADY_ENCRYPTED_ASSET_DIR":    "/data/secure",
		"THREADY_ASSET_DEDUP":            "false",
		"THREADY_ASSET_SERVICE_URL":      "http://assets:8090",

		"HERALD_MTPROTO_APP_ID":       "1234567",
		"HERALD_MTPROTO_APP_HASH":     "abcdef0123456789abcdef0123456789",
		"HERALD_MTPROTO_PHONE":        "+15551234567",
		"HERALD_MTPROTO_PASSWORD":     "cloud-2fa-pass",
		"HERALD_MTPROTO_SESSION_FILE": "/secure/mtproto.session",
		"HERALD_TGRAM_BOT_TOKEN":      "123456:AA-TELEGRAM-BOT-TOKEN",
		"HERALD_TGRAM_CHAT_ID":        "-1001234567890",
		"HERALD_TGRAM_LIVE_INBOUND":   "true",
		"HERALD_OPERATOR_IDS":         "111,222",
		"HERALD_MAX_BOT_TOKEN":        "max-bot-token",
		"HERALD_MAX_CHAT_ID":          "max-chat",

		"THREADY_MESSENGER_SIGNIN_MODE": "noninteractive",
		"THREADY_POLL_INTERVAL":         "2m",
		"THREADY_REPLY_ACCOUNT":         "robot",

		"THREADY_BOBA_URL":             "http://boba:8000",
		"THREADY_METUBE_URL":           "http://metube:8081",
		"THREADY_DOWNLOAD_CONCURRENCY": "8",

		"THREADY_EVENTBUS_BACKEND": "nats",
		"THREADY_NATS_URL":         "nats://nats:4222",
		"THREADY_NATS_STREAM":      "thready-prod",

		"THREADY_WORKERS":           "128",
		"THREADY_RETRY_MAX":         "9",
		"THREADY_RETRY_BASE":        "3s",
		"THREADY_RETRY_FACTOR":      "1.5",
		"THREADY_RETRY_CAP":         "10m",
		"THREADY_POST_TIMEOUT":      "45m",
		"THREADY_SKILL_CONCURRENCY": "16",

		"THREADY_JWT_SIGNING_ALG":      "RS256",
		"THREADY_JWT_PRIVATE_KEY_PATH": "/secure/jwt.pem",
		"THREADY_JWT_PUBLIC_KEY_PATH":  "/secure/jwt.pub.pem",
		"THREADY_ACCESS_TOKEN_TTL":     "10m",
		"THREADY_REFRESH_TOKEN_TTL":    "72h",
		"THREADY_IDLE_TIMEOUT":         "20m",
		"THREADY_PASSWORD_MIN_LEN":     "16",
		"THREADY_ARGON2_MEMORY_KIB":    "131072",
		"THREADY_API_KEY_HASH_PEPPER":  "api-key-pepper-secret",
		"THREADY_ENCRYPTION_KEY":       "0123456789abcdef0123456789abcdef",

		"OTEL_EXPORTER_OTLP_ENDPOINT": "http://otel:4317",
		"THREADY_METRICS_ADDR":        "0.0.0.0:9191",
		"THREADY_CLICKHOUSE_DSN":      "clickhouse://user:pw@ch:9000/thready",
		"THREADY_AUDIT_RETENTION":     "4380h",
		"FIREBASE_PROJECT_ID":         "thready-prod",

		"THREADY_BILLING_MODE":   "subscription",
		"THREADY_METERING_FLUSH": "30s",

		"THREADY_DEFAULT_LOCALE": "sr-Cyrl",
		"THREADY_TRANSLATE_URL":  "http://translate:8095",
		"THREADY_BRAND_NAME":     "Thready",
		"THREADY_THEME_DEFAULT":  "dark",
	}
}

func TestLoad_FullEnvParsesEveryFieldKind(t *testing.T) {
	c, err := Load(envFunc(fullEnv()))
	if err != nil {
		t.Fatalf("Load returned error on valid full env: %v", err)
	}

	// enums
	if c.Core.Env != "production" {
		t.Errorf("Env = %q", c.Core.Env)
	}
	if c.Database.Driver != "postgres" {
		t.Errorf("Driver = %q", c.Database.Driver)
	}
	if c.Vector.Backend != "qdrant" {
		t.Errorf("Vector.Backend = %q", c.Vector.Backend)
	}
	if c.Auth.JWTSigningAlg != "RS256" {
		t.Errorf("JWTSigningAlg = %q", c.Auth.JWTSigningAlg)
	}
	if c.Branding.ThemeDefault != "dark" {
		t.Errorf("ThemeDefault = %q", c.Branding.ThemeDefault)
	}
	// ints
	if c.Deployment.RateLimitRPS != 250 {
		t.Errorf("RateLimitRPS = %d", c.Deployment.RateLimitRPS)
	}
	if c.Database.MaxOpenConns != 64 {
		t.Errorf("MaxOpenConns = %d", c.Database.MaxOpenConns)
	}
	if c.Embeddings.Dim != 768 {
		t.Errorf("Embeddings.Dim = %d", c.Embeddings.Dim)
	}
	if c.Messengers.Telegram.AppID != 1234567 {
		t.Errorf("Telegram.AppID = %d", c.Messengers.Telegram.AppID)
	}
	if c.Auth.Argon2MemoryKiB != 131072 {
		t.Errorf("Argon2MemoryKiB = %d", c.Auth.Argon2MemoryKiB)
	}
	// floats
	if c.Vision.SSIMThreshold != 0.88 {
		t.Errorf("SSIMThreshold = %v", c.Vision.SSIMThreshold)
	}
	if c.Workers.RetryFactor != 1.5 {
		t.Errorf("RetryFactor = %v", c.Workers.RetryFactor)
	}
	// bools
	if c.Deployment.HTTP3Enabled {
		t.Errorf("HTTP3Enabled should be false")
	}
	if c.Vision.OpenCVEnabled {
		t.Errorf("Vision.OpenCVEnabled should be false")
	}
	if !c.Messengers.Telegram.LiveInbound {
		t.Errorf("Telegram.LiveInbound should be true")
	}
	// durations
	if c.Deployment.RequestTimeout != 45*time.Second {
		t.Errorf("RequestTimeout = %v", c.Deployment.RequestTimeout)
	}
	if c.Auth.RefreshTokenTTL != 72*time.Hour {
		t.Errorf("RefreshTokenTTL = %v", c.Auth.RefreshTokenTTL)
	}
	if c.Cache.TTL != 20*time.Minute {
		t.Errorf("Cache.TTL = %v", c.Cache.TTL)
	}
	// urls & strings
	if c.Storage.Endpoint != "https://minio:9000" {
		t.Errorf("Storage.Endpoint = %q", c.Storage.Endpoint)
	}
	if c.EventBus.NATSURL != "nats://nats:4222" {
		t.Errorf("NATSURL = %q", c.EventBus.NATSURL)
	}
	if c.Messengers.Telegram.Phone != "+15551234567" {
		t.Errorf("Phone = %q", c.Messengers.Telegram.Phone)
	}
	// secret maps
	if c.LLM.CloudProviderKeys["OPENAI_API_KEY"] != "sk-openai-secret" {
		t.Errorf("OPENAI_API_KEY not captured: %v", c.LLM.CloudProviderKeys)
	}
	if c.LLM.CloudProviderKeys["REPLICATE_API_TOKEN"] != "r8-replicate-secret" {
		t.Errorf("REPLICATE_API_TOKEN not captured: %v", c.LLM.CloudProviderKeys)
	}
	if c.Vision.ProviderKeys["ASTICA_API_KEY"] != "astica-secret" {
		t.Errorf("ASTICA_API_KEY not captured: %v", c.Vision.ProviderKeys)
	}
	// remote block
	if c.Deployment.Remote.PortRangeStart != 21000 || c.Deployment.Remote.PortRangeEnd != 22000 {
		t.Errorf("remote port range = %d..%d", c.Deployment.Remote.PortRangeStart, c.Deployment.Remote.PortRangeEnd)
	}
}

func TestLoad_DefaultsAppliedOnEmptyEnv(t *testing.T) {
	c, err := Load(envFunc(map[string]string{}))
	if err != nil {
		t.Fatalf("Load on empty env should succeed (development defaults), got: %v", err)
	}
	checks := []struct {
		name string
		got  any
		want any
	}{
		{"Env", c.Core.Env, "development"},
		{"EnvFile", c.Core.EnvFile, "./.env"},
		{"LogLevel", c.Core.LogLevel, "info"},
		{"LogFormat(dev=text)", c.Core.LogFormat, "text"},
		{"PublicDomain", c.Core.PublicDomain, "thready.hxd3v.com"},
		{"HTTPAddr", c.Deployment.HTTPAddr, "0.0.0.0:8443"},
		{"HTTP3Enabled", c.Deployment.HTTP3Enabled, true},
		{"RequestTimeout", c.Deployment.RequestTimeout, 30 * time.Second},
		{"RateLimitRPS", c.Deployment.RateLimitRPS, 100},
		{"TLSMinVersion", c.Deployment.TLSMinVersion, "1.3"},
		{"Remote.Runtime", c.Deployment.Remote.DefaultRuntime, "podman"},
		{"Remote.PortStart", c.Deployment.Remote.PortRangeStart, 20000},
		{"DBDriver", c.Database.Driver, "sqlite"},
		{"DBMaxOpen", c.Database.MaxOpenConns, 32},
		{"DBConnMaxLifetime", c.Database.ConnMaxLifetime, 30 * time.Minute},
		{"MigrateOnBoot(dev=true)", c.Database.MigrateOnBoot, true},
		{"VectorBackend", c.Vector.Backend, "pgvector"},
		{"VectorMetric", c.Vector.Metric, "cosine"},
		{"EmbeddingDim", c.Embeddings.Dim, 1024},
		{"EmbeddingProvider", c.Embeddings.Provider, "llama"},
		{"EmbeddingBaseURL", c.Embeddings.BaseURL, "http://localhost:8080/v1"},
		{"EmbeddingModel", c.Embeddings.Model, "jina-embeddings-v2-base-code"},
		{"LLMMaxRetries", c.LLM.MaxRetries, 5},
		{"VisionProvider", c.Vision.Provider, "auto"},
		{"VisionTimeout", c.Vision.Timeout, 60},
		{"VisionMaxImageSize", c.Vision.MaxImageSize, 4096},
		{"VisionSSIM", c.Vision.SSIMThreshold, 0.95},
		{"OllamaModel", c.Vision.OllamaModel, "minicpm-v:8b"},
		{"OCRProvider", c.OCR.Provider, "none"},
		{"OCRLangs", c.OCR.Langs, "eng,rus"},
		{"CacheBackend", c.Cache.Backend, "memory"},
		{"CacheTTL", c.Cache.TTL, 10 * time.Minute},
		{"StorageBackend", c.Storage.Backend, "filesystem"},
		{"StorageBucket", c.Storage.Bucket, "thready-assets"},
		{"MediaDir", c.Storage.MediaDir, "./data/media"},
		{"EncryptedAssetDir", c.Storage.EncryptedAssetDir, "./data/secure"},
		{"SessionFile", c.Messengers.Telegram.SessionFile, "~/.config/herald/mtproto.session"},
		{"SigninMode", c.Messengers.SigninMode, "interactive"},
		{"PollInterval", c.Messengers.PollInterval, 5 * time.Minute},
		{"ReplyAccount", c.Messengers.ReplyAccount, "robot"},
		{"DownloadConcurrency", c.Downloads.Concurrency, 4},
		{"GamePlatforms", c.Downloads.GameDefaultPlatforms, "PC-Windows,PS4,Android"},
		{"EventBusBackend", c.EventBus.Backend, "inprocess"},
		{"NATSStream", c.EventBus.NATSStream, "thready"},
		{"Workers", c.Workers.Workers, 32},
		{"RetryBase", c.Workers.RetryBase, 2 * time.Second},
		{"RetryFactor", c.Workers.RetryFactor, 2.0},
		{"SkillConcurrency", c.Workers.SkillConcurrency, 8},
		{"JWTAlg", c.Auth.JWTSigningAlg, "HS256"},
		{"AccessTTL", c.Auth.AccessTokenTTL, 15 * time.Minute},
		{"RefreshTTL", c.Auth.RefreshTokenTTL, 168 * time.Hour},
		{"PasswordMinLen", c.Auth.PasswordMinLen, 12},
		{"Argon2Mem", c.Auth.Argon2MemoryKiB, 65536},
		{"MetricsAddr", c.Observability.MetricsAddr, "0.0.0.0:9090"},
		{"AuditRetention", c.Observability.AuditRetention, 8760 * time.Hour},
		{"BackupFullCron", c.Observability.BackupFullCron, "0 3 * * *"},
		{"BillingMode", c.Billing.Mode, "subscription+metered"},
		{"MeteringFlush", c.Billing.MeteringFlush, 1 * time.Minute},
		{"RetentionDefault", c.Billing.RetentionDefault, "indefinite"},
		{"DefaultLocale", c.Branding.DefaultLocale, "en"},
		{"BrandName", c.Branding.BrandName, "Thready"},
		{"BrandColor", c.Branding.BrandPrimaryColor, "#B6E376"},
		{"ThemeDefault", c.Branding.ThemeDefault, "system"},
	}
	for _, ck := range checks {
		if ck.got != ck.want {
			t.Errorf("default %s = %v, want %v", ck.name, ck.got, ck.want)
		}
	}
}

func TestConfig_SQLitePath(t *testing.T) {
	c, err := Load(envFunc(map[string]string{
		"THREADY_DB_DRIVER": "sqlite",
		"THREADY_DB_DSN":    "file:./data/thready.db?_pragma=busy_timeout(5000)",
	}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := c.Database.SQLitePath(); got != "./data/thready.db" {
		t.Errorf("SQLitePath() = %q, want %q", got, "./data/thready.db")
	}
	// non-sqlite yields empty
	pg := DatabaseConfig{Driver: "postgres", DSN: "postgres://x"}
	if got := pg.SQLitePath(); got != "" {
		t.Errorf("SQLitePath() for postgres = %q, want empty", got)
	}
}

func TestConfig_RedactionHidesSecrets(t *testing.T) {
	const botToken = "123456:AA-TELEGRAM-BOT-TOKEN"
	const jwtSecret = "super-secret-jwt-signing-key-32b!!"
	const openaiKey = "sk-openai-DO-NOT-LEAK"
	c, err := Load(envFunc(map[string]string{
		"HERALD_TGRAM_BOT_TOKEN": botToken,
		"THREADY_JWT_SECRET":     jwtSecret,
		"THREADY_ENCRYPTION_KEY": "0123456789abcdef0123456789abcdef",
		"OPENAI_API_KEY":         openaiKey,
		"THREADY_DB_DSN":         "postgres://u:mypassword@db/thready",
		"THREADY_BRAND_NAME":     "Thready",
		"THREADY_HTTP_ADDR":      "0.0.0.0:8443",
	}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	s := c.String()

	// Secrets must be ABSENT from String().
	for _, secret := range []string{botToken, jwtSecret, openaiKey, "mypassword"} {
		if strings.Contains(s, secret) {
			t.Errorf("String() leaked secret %q:\n%s", secret, s)
		}
	}
	// Redaction marker present, non-secrets present.
	if !strings.Contains(s, redactedMask) {
		t.Errorf("String() should contain the redaction mask %q", redactedMask)
	}
	if !strings.Contains(s, "Thready") {
		t.Errorf("String() should contain non-secret brand name")
	}
	if !strings.Contains(s, "0.0.0.0:8443") {
		t.Errorf("String() should contain non-secret HTTP addr")
	}

	// The original Config must be unchanged by redaction.
	if c.Auth.JWTSecret != jwtSecret {
		t.Errorf("Redacted() mutated the original: JWTSecret = %q", c.Auth.JWTSecret)
	}
	r := c.Redacted()
	if r.Auth.JWTSecret != redactedMask {
		t.Errorf("Redacted().Auth.JWTSecret = %q, want mask", r.Auth.JWTSecret)
	}
	if r.LLM.CloudProviderKeys["OPENAI_API_KEY"] != redactedMask {
		t.Errorf("Redacted() OPENAI_API_KEY = %q, want mask", r.LLM.CloudProviderKeys["OPENAI_API_KEY"])
	}
	if r.Branding.BrandName != "Thready" {
		t.Errorf("Redacted() must preserve non-secret BrandName, got %q", r.Branding.BrandName)
	}
}

// TestLoad_RoundTripDocumentedDevExample round-trips the Appendix B.1
// development .env skeleton through ParseDotEnv -> Load.
func TestLoad_RoundTripDocumentedDevExample(t *testing.T) {
	const devEnv = `THREADY_ENV=development
THREADY_HTTP_ADDR=0.0.0.0:8443
THREADY_LOG_LEVEL=debug
THREADY_LOG_FORMAT=text
THREADY_DB_DRIVER=sqlite
THREADY_DB_DSN=file:./data/thready.db?_pragma=busy_timeout(5000)
THREADY_DB_MIGRATE_ON_BOOT=true
THREADY_VECTOR_BACKEND=pgvector
THREADY_VECTOR_DSN=postgres://thready:thready@localhost:5432/thready?sslmode=disable
THREADY_EMBEDDING_DIM=1024
HELIX_EMBEDDING_PROVIDER=llama
THREADY_EMBEDDING_BASE_URL=http://localhost:8080/v1
THREADY_EMBEDDING_MODEL=jina-embeddings-v2-base-code
THREADY_EVENTBUS_BACKEND=inprocess
THREADY_CACHE_BACKEND=memory
THREADY_STORAGE_BACKEND=filesystem
THREADY_MEDIA_DIR=./data/media
THREADY_JWT_SIGNING_ALG=HS256
THREADY_JWT_SECRET=dev-only-change-me-32-bytes-minimum
HERALD_MTPROTO_APP_ID=
HERALD_MTPROTO_APP_HASH=
HERALD_MTPROTO_PHONE=
`
	m, err := ParseDotEnv(strings.NewReader(devEnv))
	if err != nil {
		t.Fatalf("ParseDotEnv: %v", err)
	}
	c, err := Load(envFunc(m))
	if err != nil {
		t.Fatalf("Load(dev example) should succeed, got: %v", err)
	}
	if c.Core.Env != "development" {
		t.Errorf("Env = %q", c.Core.Env)
	}
	if c.Database.Driver != "sqlite" {
		t.Errorf("Driver = %q", c.Database.Driver)
	}
	if c.Database.SQLitePath() != "./data/thready.db" {
		t.Errorf("SQLitePath = %q", c.Database.SQLitePath())
	}
	if c.Embeddings.Provider != "llama" {
		t.Errorf("Embedding provider = %q", c.Embeddings.Provider)
	}
	if c.EventBus.Backend != "inprocess" {
		t.Errorf("EventBus = %q", c.EventBus.Backend)
	}
	if c.Auth.JWTSecret != "dev-only-change-me-32-bytes-minimum" {
		t.Errorf("JWTSecret round-trip failed: %q", c.Auth.JWTSecret)
	}
}

// TestLoad_RoundTripDocumentedProdExample round-trips the Appendix B.3
// production skeleton, substituting concrete values for the ${...} secret
// placeholders (which a real shell would have expanded).
func TestLoad_RoundTripDocumentedProdExample(t *testing.T) {
	const prodEnv = `THREADY_ENV=production
THREADY_HTTP_ADDR=0.0.0.0:8443
THREADY_HTTP3_ENABLED=true
THREADY_LOG_LEVEL=info
THREADY_LOG_FORMAT=json
THREADY_PUBLIC_DOMAIN=thready.hxd3v.com
THREADY_TLS_MIN_VERSION=1.3
LETS_ENCRYPT_EMAIL=ops@hxd3v.com
THREADY_DB_DRIVER=postgres
THREADY_DB_DSN=postgres://thready:realpw@db:5432/thready?sslmode=require
THREADY_DB_MIGRATE_ON_BOOT=false
THREADY_DB_MAX_OPEN_CONNS=64
THREADY_DB_PARTITIONING=true
THREADY_VECTOR_BACKEND=pgvector
THREADY_VECTOR_DSN=postgres://thready:realpw@db:5432/thready?sslmode=require
THREADY_EMBEDDING_DIM=1024
THREADY_VECTOR_INDEX=hnsw
HELIX_EMBEDDING_PROVIDER=llama
THREADY_EMBEDDING_BASE_URL=http://llm:8080/v1
THREADY_EVENTBUS_BACKEND=nats
THREADY_NATS_URL=nats://nats:4222
THREADY_WORKERS=64
THREADY_CACHE_BACKEND=redis
THREADY_CACHE_REDIS_URL=redis://cache:6379/0
THREADY_STORAGE_BACKEND=minio
THREADY_STORAGE_ENDPOINT=https://minio:9000
THREADY_STORAGE_BUCKET=thready-assets
THREADY_JWT_SIGNING_ALG=RS256
THREADY_JWT_PRIVATE_KEY_PATH=/secure/jwt.pem
THREADY_JWT_PUBLIC_KEY_PATH=/secure/jwt.pub.pem
THREADY_ACCESS_TOKEN_TTL=15m
THREADY_REFRESH_TOKEN_TTL=168h
THREADY_MFA_REQUIRED_TIERS=root,account_admin
THREADY_ENCRYPTION_KEY=0123456789abcdef0123456789abcdef
THREADY_ENCRYPTED_ASSET_DIR=/data/secure
OTEL_EXPORTER_OTLP_ENDPOINT=http://otel:4317
THREADY_METRICS_ADDR=0.0.0.0:9090
THREADY_CLICKHOUSE_DSN=clickhouse://ch:9000/thready
THREADY_AUDIT_RETENTION=8760h
THREADY_RETENTION_DEFAULT=indefinite
THREADY_BILLING_MODE=subscription+metered
THREADY_DEFAULT_LOCALE=en
THREADY_MESSENGER_SIGNIN_MODE=noninteractive
HERALD_MTPROTO_SESSION_FILE=/secure/mtproto.session
`
	m, err := ParseDotEnv(strings.NewReader(prodEnv))
	if err != nil {
		t.Fatalf("ParseDotEnv: %v", err)
	}
	c, err := Load(envFunc(m))
	if err != nil {
		t.Fatalf("Load(prod example) should succeed, got: %v", err)
	}
	if c.Core.Env != "production" {
		t.Errorf("Env = %q", c.Core.Env)
	}
	if c.Database.MigrateOnBoot {
		t.Errorf("prod MigrateOnBoot should be false")
	}
	if c.Auth.JWTSigningAlg != "RS256" || c.Auth.JWTPrivateKeyPath == "" {
		t.Errorf("prod JWT config wrong: %+v", c.Auth)
	}
	if c.EventBus.Backend != "nats" || c.EventBus.NATSURL == "" {
		t.Errorf("prod NATS config wrong")
	}
	// String() must not leak the DSN password even for the prod config.
	if strings.Contains(c.String(), "realpw") {
		t.Errorf("String() leaked DSN password")
	}
}
