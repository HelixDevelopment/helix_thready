package threadyconfig

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Getenv is the environment-lookup signature Load depends on. It mirrors
// os.Getenv: an unset variable yields "".
type Getenv = func(string) string

// parser reads and type-converts env values, accumulating format errors so they
// can be reported together.
type parser struct {
	get  func(string) string
	env  string
	errs []error
}

func (p *parser) addf(format string, a ...any) {
	p.errs = append(p.errs, fmt.Errorf(format, a...))
}

// raw returns the trimmed value, or "" if unset.
func (p *parser) raw(key string) string {
	return strings.TrimSpace(p.get(key))
}

// str returns the value or def when unset.
func (p *parser) str(key, def string) string {
	if v := p.raw(key); v != "" {
		return v
	}
	return def
}

func (p *parser) integer(key string, def int) int {
	v := p.raw(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		p.addf("%s: invalid integer %q", key, v)
		return def
	}
	return n
}

func (p *parser) float(key string, def float64) float64 {
	v := p.raw(key)
	if v == "" {
		return def
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		p.addf("%s: invalid number %q", key, v)
		return def
	}
	return f
}

func (p *parser) boolean(key string, def bool) bool {
	v := p.raw(key)
	if v == "" {
		return def
	}
	b, ok := parseBool(v)
	if !ok {
		p.addf("%s: invalid boolean %q (want true/false/1/0/yes/no/on/off)", key, v)
		return def
	}
	return b
}

func (p *parser) duration(key string, def time.Duration) time.Duration {
	v := p.raw(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		p.addf("%s: invalid duration %q (e.g. 30s, 5m, 168h)", key, v)
		return def
	}
	return d
}

// enum returns the value validated against allowed; def must be a member.
func (p *parser) enum(key, def string, allowed ...string) string {
	v := p.str(key, def)
	for _, a := range allowed {
		if v == a {
			return v
		}
	}
	p.addf("%s: %q is not one of [%s]", key, v, strings.Join(allowed, ", "))
	return v
}

// urlStr returns the value (or def), validating URL shape when non-empty.
func (p *parser) urlStr(key, def string) string {
	v := p.str(key, def)
	if v == "" {
		return v
	}
	if err := validateURL(v); err != nil {
		p.addf("%s: %v (%q)", key, err, v)
	}
	return v
}

func parseBool(s string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "t", "true", "yes", "y", "on":
		return true, true
	case "0", "f", "false", "no", "n", "off":
		return false, true
	}
	return false, false
}

func validateURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid URL: %v", err)
	}
	if u.Scheme == "" {
		return fmt.Errorf("missing URL scheme")
	}
	if u.Host == "" {
		return fmt.Errorf("missing URL host")
	}
	return nil
}

// LoadFromEnv loads configuration from the process environment (os.Getenv).
func LoadFromEnv() (*Config, error) {
	return Load(os.Getenv)
}

// Load parses the injected environment into a typed Config, applies documented
// defaults, and validates it. On any format or required-field problem it
// returns a nil Config and a *MultiError aggregating every problem found; each
// message names the offending variable.
func Load(getenv Getenv) (*Config, error) {
	if getenv == nil {
		getenv = func(string) string { return "" }
	}
	p := &parser{get: getenv}

	c := &Config{}

	// --- Core ---
	env := p.enum("THREADY_ENV", "development", "development", "staging", "production", "test")
	p.env = env
	isProd := env == "production"
	logFormatDefault := "json"
	if env == "development" {
		logFormatDefault = "text"
	}
	c.Core = CoreConfig{
		Env:           env,
		EnvFile:       p.str("THREADY_ENV_FILE", "./.env"),
		LogLevel:      p.enum("THREADY_LOG_LEVEL", "info", "trace", "debug", "info", "warn", "warning", "error", "fatal", "panic"),
		LogFormat:     p.enum("THREADY_LOG_FORMAT", logFormatDefault, "json", "text"),
		PublicDomain:  p.str("THREADY_PUBLIC_DOMAIN", "thready.hxd3v.com"),
		PublicBaseURL: p.urlStr("THREADY_PUBLIC_BASE_URL", "https://thready.hxd3v.com"),
	}

	// --- Deployment ---
	c.Deployment = DeploymentConfig{
		HTTPAddr:             p.str("THREADY_HTTP_ADDR", "0.0.0.0:8443"),
		HTTP3Enabled:         p.boolean("THREADY_HTTP3_ENABLED", true),
		HTTPCompression:      p.str("THREADY_HTTP_COMPRESSION", "br,gzip"),
		RequestTimeout:       p.duration("THREADY_REQUEST_TIMEOUT", 30*time.Second),
		RateLimitRPS:         p.integer("THREADY_RATE_LIMIT_RPS", 100),
		CORSOrigins:          p.str("THREADY_CORS_ORIGINS", ""),
		PortPrefix:           p.str("THREADY_PORT_PREFIX", ""),
		TLSMinVersion:        p.enum("THREADY_TLS_MIN_VERSION", "1.3", "1.2", "1.3"),
		LetsEncryptEmail:     p.str("LETS_ENCRYPT_EMAIL", ""),
		LetsEncryptChallenge: p.enum("LETS_ENCRYPT_CHALLENGE", "http-01", "http-01", "dns-01", "tls-alpn-01"),
		Remote: RemoteConfig{
			Enabled:           p.boolean("CONTAINERS_REMOTE_ENABLED", false),
			DefaultSSHUser:    p.str("CONTAINERS_REMOTE_DEFAULT_SSH_USER", "deploy"),
			DefaultRuntime:    p.enum("CONTAINERS_REMOTE_DEFAULT_RUNTIME", "podman", "podman", "docker"),
			DefaultSSHKey:     p.str("CONTAINERS_REMOTE_DEFAULT_SSH_KEY", ""),
			PortRangeStart:    p.integer("CONTAINERS_REMOTE_PORT_RANGE_START", 20000),
			PortRangeEnd:      p.integer("CONTAINERS_REMOTE_PORT_RANGE_END", 30000),
			ConnectTimeout:    p.duration("CONTAINERS_REMOTE_CONNECT_TIMEOUT", 10*time.Second),
			CommandTimeout:    p.duration("CONTAINERS_REMOTE_COMMAND_TIMEOUT", 5*time.Minute),
			SSHControlMaster:  p.boolean("CONTAINERS_REMOTE_SSH_CONTROL_MASTER", true),
			SSHControlPersist: p.duration("CONTAINERS_REMOTE_SSH_CONTROL_PERSIST", 60*time.Second),
			SSHMaxConnections: p.integer("CONTAINERS_REMOTE_SSH_MAX_CONNECTIONS", 10),
			Scheduler:         p.enum("CONTAINERS_REMOTE_SCHEDULER", "roundrobin", "roundrobin", "leastloaded"),
			VolumeType:        p.enum("CONTAINERS_REMOTE_VOLUME_TYPE", "volume", "volume", "bind", "tmpfs"),
		},
	}

	// --- Database ---
	c.Database = DatabaseConfig{
		Driver:          p.enum("THREADY_DB_DRIVER", "sqlite", "sqlite", "postgres"),
		DSN:             p.str("THREADY_DB_DSN", ""),
		MaxOpenConns:    p.integer("THREADY_DB_MAX_OPEN_CONNS", 32),
		MaxIdleConns:    p.integer("THREADY_DB_MAX_IDLE_CONNS", 8),
		ConnMaxLifetime: p.duration("THREADY_DB_CONN_MAX_LIFETIME", 30*time.Minute),
		MigrateOnBoot:   p.boolean("THREADY_DB_MIGRATE_ON_BOOT", !isProd),
		Partitioning:    p.boolean("THREADY_DB_PARTITIONING", isProd),
	}

	// --- Vector ---
	c.Vector = VectorConfig{
		Backend:   p.enum("THREADY_VECTOR_BACKEND", "pgvector", "pgvector", "qdrant"),
		DSN:       p.str("THREADY_VECTOR_DSN", ""),
		Metric:    p.enum("THREADY_VECTOR_METRIC", "cosine", "cosine", "l2", "euclidean", "dot", "ip"),
		Index:     p.enum("THREADY_VECTOR_INDEX", "hnsw", "hnsw", "ivfflat", "flat"),
		QdrantURL: p.urlStr("THREADY_QDRANT_URL", ""),
	}

	// --- Embeddings ---
	c.Embeddings = EmbeddingsConfig{
		Provider: p.enum("HELIX_EMBEDDING_PROVIDER", "llama", "llama"),
		BaseURL:  p.urlStr("THREADY_EMBEDDING_BASE_URL", "http://localhost:8080/v1"),
		Model:    p.str("THREADY_EMBEDDING_MODEL", "jina-embeddings-v2-base-code"),
		Dim:      p.integer("THREADY_EMBEDDING_DIM", 1024),
		APIKey:   p.str("THREADY_EMBEDDING_API_KEY", ""),
	}

	// --- LLM ---
	c.LLM = LLMConfig{
		BaseURL:           p.urlStr("HELIX_LLM_BASE_URL", ""),
		Model:             p.str("HELIX_LLM_MODEL", ""),
		CodeModel:         p.str("HELIX_LLM_CODE_MODEL", ""),
		MaxRetries:        p.integer("THREADY_LLM_MAX_RETRIES", 5),
		CircuitBreaker:    p.boolean("THREADY_LLM_CIRCUIT_BREAKER", true),
		CloudProviderKeys: collectKeys(getenv, CloudLLMProviderEnvVars),
	}

	// --- Vision ---
	c.Vision = VisionConfig{
		Provider:           p.enum("HELIX_VISION_PROVIDER", "auto", "auto", "ollama", "llamacpp", "remote", "opencv", "astica", "kimi", "stepfun"),
		Timeout:            p.integer("HELIX_VISION_TIMEOUT", 60),
		MaxImageSize:       p.integer("HELIX_VISION_MAX_IMAGE_SIZE", 4096),
		OpenCVEnabled:      p.boolean("HELIX_VISION_OPENCV_ENABLED", true),
		SSIMThreshold:      p.float("HELIX_VISION_SSIM_THRESHOLD", 0.95),
		Hosts:              p.str("HELIX_VISION_HOSTS", ""),
		User:               p.str("HELIX_VISION_USER", ""),
		OllamaURL:          p.urlStr("HELIX_OLLAMA_URL", "http://localhost:11434"),
		OllamaModel:        p.str("HELIX_OLLAMA_MODEL", "minicpm-v:8b"),
		LlamaCppRPCEnabled: p.boolean("HELIX_LLAMACPP_RPC_ENABLED", false),
		LlamaCppRPCWorkers: p.str("HELIX_LLAMACPP_RPC_WORKERS", ""),
		LlamaCppRPCModel:   p.str("HELIX_LLAMACPP_RPC_MODEL", ""),
		ProviderKeys:       collectKeys(getenv, VisionProviderKeyEnvVars),
	}

	// --- OCR ---
	c.OCR = OCRConfig{
		Provider: p.str("THREADY_OCR_PROVIDER", "none"),
		Langs:    p.str("THREADY_OCR_LANGS", "eng,rus"),
	}

	// --- Cache ---
	c.Cache = CacheConfig{
		Backend:  p.enum("THREADY_CACHE_BACKEND", "memory", "memory", "redis"),
		RedisURL: p.urlStr("THREADY_CACHE_REDIS_URL", ""),
		TTL:      p.duration("THREADY_CACHE_TTL", 10*time.Minute),
	}

	// --- Storage ---
	c.Storage = StorageConfig{
		Backend:            p.enum("THREADY_STORAGE_BACKEND", "filesystem", "filesystem", "minio", "s3"),
		Endpoint:           p.urlStr("THREADY_STORAGE_ENDPOINT", ""),
		Bucket:             p.str("THREADY_STORAGE_BUCKET", "thready-assets"),
		AccessKey:          p.str("THREADY_STORAGE_ACCESS_KEY", ""),
		SecretKey:          p.str("THREADY_STORAGE_SECRET_KEY", ""),
		SignedURLTTL:       p.duration("THREADY_STORAGE_SIGNED_URL_TTL", 15*time.Minute),
		MediaDir:           p.str("THREADY_MEDIA_DIR", "./data/media"),
		WebRenditionSuffix: p.str("THREADY_WEB_RENDITION_SUFFIX", "-web"),
		EncryptedAssetDir:  p.str("THREADY_ENCRYPTED_ASSET_DIR", "./data/secure"),
		AssetDedup:         p.boolean("THREADY_ASSET_DEDUP", true),
		AssetServiceURL:    p.urlStr("THREADY_ASSET_SERVICE_URL", ""),
	}

	// --- Messengers ---
	c.Messengers = MessengersConfig{
		Telegram: TelegramConfig{
			AppID:       p.integer("HERALD_MTPROTO_APP_ID", 0),
			AppHash:     p.str("HERALD_MTPROTO_APP_HASH", ""),
			Phone:       p.str("HERALD_MTPROTO_PHONE", ""),
			Password:    p.str("HERALD_MTPROTO_PASSWORD", ""),
			SessionFile: p.str("HERALD_MTPROTO_SESSION_FILE", "~/.config/herald/mtproto.session"),
			BotToken:    p.str("HERALD_TGRAM_BOT_TOKEN", ""),
			ChatID:      p.str("HERALD_TGRAM_CHAT_ID", ""),
			LiveInbound: p.boolean("HERALD_TGRAM_LIVE_INBOUND", false),
		},
		Max: MaxConfig{
			BotToken: p.str("HERALD_MAX_BOT_TOKEN", ""),
			ChatID:   p.str("HERALD_MAX_CHAT_ID", ""),
		},
		SigninMode:   p.enum("THREADY_MESSENGER_SIGNIN_MODE", "interactive", "interactive", "noninteractive"),
		PollInterval: p.duration("THREADY_POLL_INTERVAL", 5*time.Minute),
		ReplyAccount: p.str("THREADY_REPLY_ACCOUNT", "robot"),
		OperatorIDs:  p.str("HERALD_OPERATOR_IDS", ""),
	}

	// --- Downloads ---
	c.Downloads = DownloadsConfig{
		BobaURL:              p.urlStr("THREADY_BOBA_URL", ""),
		BobaCallbackURL:      p.urlStr("THREADY_BOBA_CALLBACK_URL", ""),
		MetubeURL:            p.urlStr("THREADY_METUBE_URL", ""),
		MetubeWebhookURL:     p.urlStr("THREADY_METUBE_WEBHOOK_URL", ""),
		DownloadManagerURL:   p.urlStr("THREADY_DOWNLOAD_MANAGER_URL", ""),
		Concurrency:          p.integer("THREADY_DOWNLOAD_CONCURRENCY", 4),
		GameDefaultPlatforms: p.str("THREADY_GAME_DEFAULT_PLATFORMS", "PC-Windows,PS4,Android"),
		SoftwareDefaultOS:    p.str("THREADY_SOFTWARE_DEFAULT_OS", "Windows,Linux,macOS"),
	}

	// --- EventBus ---
	c.EventBus = EventBusConfig{
		Backend:    p.enum("THREADY_EVENTBUS_BACKEND", "inprocess", "inprocess", "nats"),
		NATSURL:    p.urlStr("THREADY_NATS_URL", ""),
		NATSStream: p.str("THREADY_NATS_STREAM", "thready"),
	}

	// --- Workers ---
	c.Workers = WorkersConfig{
		Workers:          p.integer("THREADY_WORKERS", 32),
		RetryMax:         p.integer("THREADY_RETRY_MAX", 5),
		RetryBase:        p.duration("THREADY_RETRY_BASE", 2*time.Second),
		RetryFactor:      p.float("THREADY_RETRY_FACTOR", 2.0),
		RetryCap:         p.duration("THREADY_RETRY_CAP", 5*time.Minute),
		PostTimeout:      p.duration("THREADY_POST_TIMEOUT", 30*time.Minute),
		SkillConcurrency: p.integer("THREADY_SKILL_CONCURRENCY", 8),
	}

	// --- Auth ---
	c.Auth = AuthConfig{
		JWTSigningAlg:     p.enum("THREADY_JWT_SIGNING_ALG", "HS256", "HS256", "RS256", "EdDSA"),
		JWTSecret:         p.str("THREADY_JWT_SECRET", ""),
		JWTPrivateKeyPath: p.str("THREADY_JWT_PRIVATE_KEY_PATH", ""),
		JWTPublicKeyPath:  p.str("THREADY_JWT_PUBLIC_KEY_PATH", ""),
		AccessTokenTTL:    p.duration("THREADY_ACCESS_TOKEN_TTL", 15*time.Minute),
		RefreshTokenTTL:   p.duration("THREADY_REFRESH_TOKEN_TTL", 168*time.Hour),
		IdleTimeout:       p.duration("THREADY_IDLE_TIMEOUT", 30*time.Minute),
		MFARequiredTiers:  p.str("THREADY_MFA_REQUIRED_TIERS", "root,account_admin"),
		PasswordMinLen:    p.integer("THREADY_PASSWORD_MIN_LEN", 12),
		Argon2MemoryKiB:   p.integer("THREADY_ARGON2_MEMORY_KIB", 65536),
		APIKeyHashPepper:  p.str("THREADY_API_KEY_HASH_PEPPER", ""),
		EncryptionKey:     p.str("THREADY_ENCRYPTION_KEY", ""),
	}

	// --- Observability ---
	c.Observability = ObservabilityConfig{
		OTLPEndpoint:          p.urlStr("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
		MetricsAddr:           p.str("THREADY_METRICS_ADDR", "0.0.0.0:9090"),
		ClickHouseDSN:         p.str("THREADY_CLICKHOUSE_DSN", ""),
		AuditRetention:        p.duration("THREADY_AUDIT_RETENTION", 8760*time.Hour),
		BackupFullCron:        p.str("THREADY_BACKUP_FULL_CRON", "0 3 * * *"),
		BackupIncrementalCron: p.str("THREADY_BACKUP_INCREMENTAL_CRON", "0 * * * *"),
		FirebaseProjectID:     p.str("FIREBASE_PROJECT_ID", ""),
	}

	// --- Billing ---
	c.Billing = BillingConfig{
		Mode:             p.str("THREADY_BILLING_MODE", "subscription+metered"),
		MeteringFlush:    p.duration("THREADY_METERING_FLUSH", 1*time.Minute),
		RetentionDefault: p.str("THREADY_RETENTION_DEFAULT", "indefinite"),
	}

	// --- Branding ---
	c.Branding = BrandingConfig{
		DefaultLocale:     p.str("THREADY_DEFAULT_LOCALE", "en"),
		TranslateURL:      p.urlStr("THREADY_TRANSLATE_URL", ""),
		BrandName:         p.str("THREADY_BRAND_NAME", "Thready"),
		BrandPrimaryColor: p.str("THREADY_BRAND_PRIMARY_COLOR", "#B6E376"),
		BrandLogoPath:     p.str("THREADY_BRAND_LOGO_PATH", "./assets/Logo.png"),
		BrandSlogan:       p.str("THREADY_BRAND_SLOGAN", "Made with love ♥ by Helix Development"),
		ThemeDefault:      p.enum("THREADY_THEME_DEFAULT", "system", "system", "light", "dark"),
	}

	// Cross-field / required validation appends to p.errs.
	p.validate(c)

	if len(p.errs) > 0 {
		return nil, &MultiError{Errors: p.errs}
	}
	return c, nil
}

// collectKeys reads each env var name and, when set, records it in the map.
func collectKeys(getenv func(string) string, names []string) map[string]string {
	m := make(map[string]string)
	for _, k := range names {
		if v := strings.TrimSpace(getenv(k)); v != "" {
			m[k] = v
		}
	}
	return m
}
