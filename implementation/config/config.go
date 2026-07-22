// Package threadyconfig is the typed, validated configuration loader for Helix
// Thready. It parses the documented THREADY_/HELIX_/HERALD_ environment
// variables (see docs/public/research/mvp/user-guides/configuration.md,
// Appendix A) into a strongly-typed Config grouped by subsystem, applies the
// documented defaults, validates format and required-in-production constraints
// returning an aggregated error, and redacts secret fields from its string
// forms.
//
// The package is stdlib-only and has no sibling-module imports; build every
// command with GOWORK=off.
package threadyconfig

import (
	"fmt"
	"strings"
	"time"
)

// Config is the fully-resolved, typed configuration, grouped by subsystem.
type Config struct {
	Core          CoreConfig
	Deployment    DeploymentConfig
	Database      DatabaseConfig
	Vector        VectorConfig
	Embeddings    EmbeddingsConfig
	LLM           LLMConfig
	Vision        VisionConfig
	OCR           OCRConfig
	Cache         CacheConfig
	Storage       StorageConfig
	Messengers    MessengersConfig
	Downloads     DownloadsConfig
	EventBus      EventBusConfig
	Workers       WorkersConfig
	Auth          AuthConfig
	Observability ObservabilityConfig
	Billing       BillingConfig
	Branding      BrandingConfig
}

// CoreConfig holds process-wide runtime selectors.
type CoreConfig struct {
	Env           string // THREADY_ENV
	EnvFile       string // THREADY_ENV_FILE
	LogLevel      string // THREADY_LOG_LEVEL
	LogFormat     string // THREADY_LOG_FORMAT
	PublicDomain  string // THREADY_PUBLIC_DOMAIN
	PublicBaseURL string // THREADY_PUBLIC_BASE_URL
}

// DeploymentConfig holds bind addresses, ports, TLS and remote-container knobs.
type DeploymentConfig struct {
	HTTPAddr             string        // THREADY_HTTP_ADDR
	HTTP3Enabled         bool          // THREADY_HTTP3_ENABLED
	HTTPCompression      string        // THREADY_HTTP_COMPRESSION
	RequestTimeout       time.Duration // THREADY_REQUEST_TIMEOUT
	RateLimitRPS         int           // THREADY_RATE_LIMIT_RPS
	CORSOrigins          string        // THREADY_CORS_ORIGINS
	PortPrefix           string        // THREADY_PORT_PREFIX
	TLSMinVersion        string        // THREADY_TLS_MIN_VERSION
	LetsEncryptEmail     string        // LETS_ENCRYPT_EMAIL
	LetsEncryptChallenge string        // LETS_ENCRYPT_CHALLENGE
	Remote               RemoteConfig  // CONTAINERS_REMOTE_*
}

// RemoteConfig holds the CONTAINERS_REMOTE_* remote-distribution knobs.
type RemoteConfig struct {
	Enabled           bool          // CONTAINERS_REMOTE_ENABLED
	DefaultSSHUser    string        // CONTAINERS_REMOTE_DEFAULT_SSH_USER
	DefaultRuntime    string        // CONTAINERS_REMOTE_DEFAULT_RUNTIME
	DefaultSSHKey     string        // CONTAINERS_REMOTE_DEFAULT_SSH_KEY
	PortRangeStart    int           // CONTAINERS_REMOTE_PORT_RANGE_START
	PortRangeEnd      int           // CONTAINERS_REMOTE_PORT_RANGE_END
	ConnectTimeout    time.Duration // CONTAINERS_REMOTE_CONNECT_TIMEOUT
	CommandTimeout    time.Duration // CONTAINERS_REMOTE_COMMAND_TIMEOUT
	SSHControlMaster  bool          // CONTAINERS_REMOTE_SSH_CONTROL_MASTER
	SSHControlPersist time.Duration // CONTAINERS_REMOTE_SSH_CONTROL_PERSIST
	SSHMaxConnections int           // CONTAINERS_REMOTE_SSH_MAX_CONNECTIONS
	Scheduler         string        // CONTAINERS_REMOTE_SCHEDULER
	VolumeType        string        // CONTAINERS_REMOTE_VOLUME_TYPE
}

// DatabaseConfig holds the relational-store settings.
type DatabaseConfig struct {
	Driver          string        // THREADY_DB_DRIVER
	DSN             string        // THREADY_DB_DSN (secret — contains credentials)
	MaxOpenConns    int           // THREADY_DB_MAX_OPEN_CONNS
	MaxIdleConns    int           // THREADY_DB_MAX_IDLE_CONNS
	ConnMaxLifetime time.Duration // THREADY_DB_CONN_MAX_LIFETIME
	MigrateOnBoot   bool          // THREADY_DB_MIGRATE_ON_BOOT
	Partitioning    bool          // THREADY_DB_PARTITIONING
}

// SQLitePath returns the on-disk SQLite file path parsed out of DSN when the
// driver is "sqlite" (stripping the "file:" scheme and any "?query"); it
// returns "" for any other driver.
func (d DatabaseConfig) SQLitePath() string {
	if d.Driver != "sqlite" {
		return ""
	}
	dsn := strings.TrimPrefix(d.DSN, "file:")
	if i := strings.IndexByte(dsn, '?'); i >= 0 {
		dsn = dsn[:i]
	}
	return dsn
}

// VectorConfig holds the vector-store settings.
type VectorConfig struct {
	Backend   string // THREADY_VECTOR_BACKEND
	DSN       string // THREADY_VECTOR_DSN (secret — contains credentials)
	Metric    string // THREADY_VECTOR_METRIC
	Index     string // THREADY_VECTOR_INDEX
	QdrantURL string // THREADY_QDRANT_URL
}

// EmbeddingsConfig holds the embedder settings.
type EmbeddingsConfig struct {
	Provider string // HELIX_EMBEDDING_PROVIDER
	BaseURL  string // THREADY_EMBEDDING_BASE_URL
	Model    string // THREADY_EMBEDDING_MODEL
	Dim      int    // THREADY_EMBEDDING_DIM
	APIKey   string // THREADY_EMBEDDING_API_KEY (secret)
}

// LLMConfig holds the local HelixLLM settings plus the cloud-provider API keys.
type LLMConfig struct {
	BaseURL        string // HELIX_LLM_BASE_URL
	Model          string // HELIX_LLM_MODEL
	CodeModel      string // HELIX_LLM_CODE_MODEL
	MaxRetries     int    // THREADY_LLM_MAX_RETRIES
	CircuitBreaker bool   // THREADY_LLM_CIRCUIT_BREAKER
	// CloudProviderKeys maps each documented cloud-LLM env var name (see
	// CloudLLMProviderEnvVars) to its value. All values are secrets.
	CloudProviderKeys map[string]string
}

// VisionConfig holds the VisionEngine settings.
type VisionConfig struct {
	Provider           string  // HELIX_VISION_PROVIDER
	Timeout            int     // HELIX_VISION_TIMEOUT (seconds)
	MaxImageSize       int     // HELIX_VISION_MAX_IMAGE_SIZE
	OpenCVEnabled      bool    // HELIX_VISION_OPENCV_ENABLED
	SSIMThreshold      float64 // HELIX_VISION_SSIM_THRESHOLD
	Hosts              string  // HELIX_VISION_HOSTS
	User               string  // HELIX_VISION_USER
	OllamaURL          string  // HELIX_OLLAMA_URL
	OllamaModel        string  // HELIX_OLLAMA_MODEL
	LlamaCppRPCEnabled bool    // HELIX_LLAMACPP_RPC_ENABLED
	LlamaCppRPCWorkers string  // HELIX_LLAMACPP_RPC_WORKERS
	LlamaCppRPCModel   string  // HELIX_LLAMACPP_RPC_MODEL
	// ProviderKeys maps each documented vision-provider key env var name (see
	// VisionProviderKeyEnvVars) to its value. All values are secrets.
	ProviderKeys map[string]string
}

// OCRConfig holds the OCR settings.
type OCRConfig struct {
	Provider string // THREADY_OCR_PROVIDER
	Langs    string // THREADY_OCR_LANGS
}

// CacheConfig holds the cache-tier settings.
type CacheConfig struct {
	Backend  string        // THREADY_CACHE_BACKEND
	RedisURL string        // THREADY_CACHE_REDIS_URL (secret — may contain credentials)
	TTL      time.Duration // THREADY_CACHE_TTL
}

// StorageConfig holds asset-store and media settings.
type StorageConfig struct {
	Backend            string        // THREADY_STORAGE_BACKEND
	Endpoint           string        // THREADY_STORAGE_ENDPOINT
	Bucket             string        // THREADY_STORAGE_BUCKET
	AccessKey          string        // THREADY_STORAGE_ACCESS_KEY (secret)
	SecretKey          string        // THREADY_STORAGE_SECRET_KEY (secret)
	SignedURLTTL       time.Duration // THREADY_STORAGE_SIGNED_URL_TTL
	MediaDir           string        // THREADY_MEDIA_DIR
	WebRenditionSuffix string        // THREADY_WEB_RENDITION_SUFFIX
	EncryptedAssetDir  string        // THREADY_ENCRYPTED_ASSET_DIR
	AssetDedup         bool          // THREADY_ASSET_DEDUP
	AssetServiceURL    string        // THREADY_ASSET_SERVICE_URL
}

// MessengersConfig holds the messenger-ingest/reply settings.
type MessengersConfig struct {
	Telegram     TelegramConfig
	Max          MaxConfig
	SigninMode   string        // THREADY_MESSENGER_SIGNIN_MODE
	PollInterval time.Duration // THREADY_POLL_INTERVAL
	ReplyAccount string        // THREADY_REPLY_ACCOUNT
	OperatorIDs  string        // HERALD_OPERATOR_IDS
}

// TelegramConfig holds the Herald MTProto (user) and Bot-API settings.
type TelegramConfig struct {
	AppID       int    // HERALD_MTPROTO_APP_ID
	AppHash     string // HERALD_MTPROTO_APP_HASH (secret)
	Phone       string // HERALD_MTPROTO_PHONE
	Password    string // HERALD_MTPROTO_PASSWORD (secret)
	SessionFile string // HERALD_MTPROTO_SESSION_FILE
	BotToken    string // HERALD_TGRAM_BOT_TOKEN (secret)
	ChatID      string // HERALD_TGRAM_CHAT_ID
	LiveInbound bool   // HERALD_TGRAM_LIVE_INBOUND
}

// MaxConfig holds the (planned/reserved) Max messenger settings.
type MaxConfig struct {
	BotToken string // HERALD_MAX_BOT_TOKEN (secret)
	ChatID   string // HERALD_MAX_CHAT_ID
}

// DownloadsConfig holds the download/torrent/video service settings.
type DownloadsConfig struct {
	BobaURL              string // THREADY_BOBA_URL
	BobaCallbackURL      string // THREADY_BOBA_CALLBACK_URL
	MetubeURL            string // THREADY_METUBE_URL
	MetubeWebhookURL     string // THREADY_METUBE_WEBHOOK_URL
	DownloadManagerURL   string // THREADY_DOWNLOAD_MANAGER_URL
	Concurrency          int    // THREADY_DOWNLOAD_CONCURRENCY
	GameDefaultPlatforms string // THREADY_GAME_DEFAULT_PLATFORMS
	SoftwareDefaultOS    string // THREADY_SOFTWARE_DEFAULT_OS
}

// EventBusConfig holds the event-transport settings.
type EventBusConfig struct {
	Backend    string // THREADY_EVENTBUS_BACKEND
	NATSURL    string // THREADY_NATS_URL
	NATSStream string // THREADY_NATS_STREAM
}

// WorkersConfig holds the worker-pool and retry/back-off settings.
type WorkersConfig struct {
	Workers          int           // THREADY_WORKERS
	RetryMax         int           // THREADY_RETRY_MAX
	RetryBase        time.Duration // THREADY_RETRY_BASE
	RetryFactor      float64       // THREADY_RETRY_FACTOR
	RetryCap         time.Duration // THREADY_RETRY_CAP
	PostTimeout      time.Duration // THREADY_POST_TIMEOUT
	SkillConcurrency int           // THREADY_SKILL_CONCURRENCY
}

// AuthConfig holds the auth/security settings.
type AuthConfig struct {
	JWTSigningAlg     string        // THREADY_JWT_SIGNING_ALG
	JWTSecret         string        // THREADY_JWT_SECRET (secret)
	JWTPrivateKeyPath string        // THREADY_JWT_PRIVATE_KEY_PATH
	JWTPublicKeyPath  string        // THREADY_JWT_PUBLIC_KEY_PATH
	AccessTokenTTL    time.Duration // THREADY_ACCESS_TOKEN_TTL
	RefreshTokenTTL   time.Duration // THREADY_REFRESH_TOKEN_TTL
	IdleTimeout       time.Duration // THREADY_IDLE_TIMEOUT
	MFARequiredTiers  string        // THREADY_MFA_REQUIRED_TIERS
	PasswordMinLen    int           // THREADY_PASSWORD_MIN_LEN
	Argon2MemoryKiB   int           // THREADY_ARGON2_MEMORY_KIB
	APIKeyHashPepper  string        // THREADY_API_KEY_HASH_PEPPER (secret)
	EncryptionKey     string        // THREADY_ENCRYPTION_KEY (secret)
}

// ObservabilityConfig holds telemetry, retention and backup settings.
type ObservabilityConfig struct {
	OTLPEndpoint          string        // OTEL_EXPORTER_OTLP_ENDPOINT
	MetricsAddr           string        // THREADY_METRICS_ADDR
	ClickHouseDSN         string        // THREADY_CLICKHOUSE_DSN (secret — contains credentials)
	AuditRetention        time.Duration // THREADY_AUDIT_RETENTION
	BackupFullCron        string        // THREADY_BACKUP_FULL_CRON
	BackupIncrementalCron string        // THREADY_BACKUP_INCREMENTAL_CRON
	FirebaseProjectID     string        // FIREBASE_PROJECT_ID
}

// BillingConfig holds billing/metering/retention settings.
type BillingConfig struct {
	Mode             string        // THREADY_BILLING_MODE
	MeteringFlush    time.Duration // THREADY_METERING_FLUSH
	RetentionDefault string        // THREADY_RETENTION_DEFAULT
}

// BrandingConfig holds locale/branding/theme settings.
type BrandingConfig struct {
	DefaultLocale     string // THREADY_DEFAULT_LOCALE
	TranslateURL      string // THREADY_TRANSLATE_URL
	BrandName         string // THREADY_BRAND_NAME
	BrandPrimaryColor string // THREADY_BRAND_PRIMARY_COLOR
	BrandLogoPath     string // THREADY_BRAND_LOGO_PATH
	BrandSlogan       string // THREADY_BRAND_SLOGAN
	ThemeDefault      string // THREADY_THEME_DEFAULT
}

// CloudLLMProviderEnvVars is the documented set of cloud-LLM credential env
// vars (Appendix A.3). Every value is treated as a secret.
var CloudLLMProviderEnvVars = []string{
	"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "GOOGLE_API_KEY", "DEEPSEEK_API_KEY",
	"GROQ_API_KEY", "MISTRAL_API_KEY", "QWEN_API_KEY", "OPENROUTER_API_KEY",
	"COHERE_API_KEY", "TOGETHER_API_KEY", "XAI_API_KEY", "PERPLEXITY_API_KEY",
	"NVIDIA_API_KEY", "CEREBRAS_API_KEY", "FIREWORKS_API_KEY", "HUGGINGFACE_API_KEY",
	"SAMBANOVA_API_KEY", "SILICONFLOW_API_KEY", "REPLICATE_API_TOKEN",
}

// VisionProviderKeyEnvVars is the documented set of vision-provider credential
// env vars (Appendix A.3). Every value is treated as a secret.
var VisionProviderKeyEnvVars = []string{"ASTICA_API_KEY", "KIMI_API_KEY", "STEPFUN_API_KEY"}

// redactedMask is the placeholder substituted for any non-empty secret value.
const redactedMask = "***REDACTED***"

func maskVal(v string) string {
	if v == "" {
		return ""
	}
	return redactedMask
}

// Redacted returns a deep-enough copy of c with every secret field (tokens,
// keys, passwords, credential-bearing DSNs/URLs) replaced by a fixed mask.
// Non-secret fields are preserved. The receiver is not mutated.
func (c *Config) Redacted() *Config {
	r := *c // copy value fields, including nested value structs

	r.Database.DSN = maskVal(c.Database.DSN)
	r.Vector.DSN = maskVal(c.Vector.DSN)
	r.Embeddings.APIKey = maskVal(c.Embeddings.APIKey)
	r.Cache.RedisURL = maskVal(c.Cache.RedisURL)
	r.Storage.AccessKey = maskVal(c.Storage.AccessKey)
	r.Storage.SecretKey = maskVal(c.Storage.SecretKey)
	r.Messengers.Telegram.AppHash = maskVal(c.Messengers.Telegram.AppHash)
	r.Messengers.Telegram.Password = maskVal(c.Messengers.Telegram.Password)
	r.Messengers.Telegram.BotToken = maskVal(c.Messengers.Telegram.BotToken)
	r.Messengers.Max.BotToken = maskVal(c.Messengers.Max.BotToken)
	r.Auth.JWTSecret = maskVal(c.Auth.JWTSecret)
	r.Auth.APIKeyHashPepper = maskVal(c.Auth.APIKeyHashPepper)
	r.Auth.EncryptionKey = maskVal(c.Auth.EncryptionKey)
	r.Observability.ClickHouseDSN = maskVal(c.Observability.ClickHouseDSN)

	if c.LLM.CloudProviderKeys != nil {
		m := make(map[string]string, len(c.LLM.CloudProviderKeys))
		for k, v := range c.LLM.CloudProviderKeys {
			m[k] = maskVal(v)
		}
		r.LLM.CloudProviderKeys = m
	}
	if c.Vision.ProviderKeys != nil {
		m := make(map[string]string, len(c.Vision.ProviderKeys))
		for k, v := range c.Vision.ProviderKeys {
			m[k] = maskVal(v)
		}
		r.Vision.ProviderKeys = m
	}
	return &r
}

// String renders the configuration with all secret fields masked, so it is safe
// to log. It never contains a raw secret value.
func (c *Config) String() string {
	// alias strips the String method to avoid infinite recursion via fmt.
	type alias Config
	r := c.Redacted()
	return fmt.Sprintf("%+v", (*alias)(r))
}
