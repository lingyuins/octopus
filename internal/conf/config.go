package conf

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lingyuins/octopus/internal/utils/log"
	"github.com/spf13/viper"
)

type Server struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
	// TrustedProxies 控制信任的反向代理 CIDR/IP 列表（逗号分隔），用于解析
	// X-Forwarded-For / X-Real-IP 取得真实客户端 IP。空值（默认）表示不信任
	// 任何代理，c.ClientIP() 只返回 TCP 直连地址（安全默认，防 XFF 伪造）。
	// 反代/Docker 部署应配置实际代理网段，例如 "172.17.0.0/16"。
	// "*" 表示信任所有来源（等价于 Gin 旧行为，仅开发用，有安全风险）。
	TrustedProxies string `mapstructure:"trusted_proxies"`
	// ExternalURL 是本服务对外的可访问基础 URL（含 scheme://host[:port]），
	// 用于 OAuth 回调地址拼接。为空时回退到 http://host:port。重启生效（engine 级配置）。
	ExternalURL string `mapstructure:"external_url"`
}

type Log struct {
	Level string `mapstructure:"level"`
}

type Database struct {
	Type string `mapstructure:"type"`
	Path string `mapstructure:"path"`
	// LogType / LogPath 为可选的独立「日志数据库」配置（仅承载 relay_logs）。
	// 二者任一为空时，日志沿用主库连接，行为与旧版完全一致（向后兼容）。
	// 配置后，relay_logs 落到独立库，可通过直接删库/断连实现秒级清理与卸载。
	LogType string `mapstructure:"log_type"`
	LogPath string `mapstructure:"log_path"`
	// SQLite 为主库且 type=sqlite 时生效的 per-connection PRAGMA 调优（见 issue #97）。
	// 日志库为 SQLite 时复用同一组值。
	SQLite SQLiteConfig `mapstructure:"sqlite"`
}

// SQLiteConfig 暴露 SQLite 运行时可调的 PRAGMA。这些值通过 glebarez/go-sqlite
// 驱动 DSN 的 _pragma 参数下发（驱动只认 _pragma/_txlock/_time_format/vfs 四种
// query 参数）。配置项默认面向低内存安全：禁用 mmap、cache 约 20MB。
type SQLiteConfig struct {
	// CacheSize 对应 PRAGMA cache_size。负值按 KB 计（如 -20000≈20MB），
	// 正值按页计（每页 4KB）。0 表示使用 internal/db.DefaultSQLiteCacheSize。
	CacheSize int `mapstructure:"cache_size"`
	// MMapSize 对应 PRAGMA mmap_size。0 表示禁用 mmap（低内存环境安全默认值，
	// 直接规避 mmap 缺页导致的磁盘 IO），正值按字节计。
	MMapSize int64 `mapstructure:"mmap_size"`
}

type Auth struct {
	JWTSecret string `mapstructure:"jwt_secret"`
}

type Relay struct {
	MaxJSONBodyBytes      int64 `mapstructure:"max_json_body_bytes"`
	MaxMultipartBodyBytes int64 `mapstructure:"max_multipart_body_bytes"`
}

// NodeTransport 描述云端节点反向传输的缓冲/超时参数。
// 留空（0）时回落到 nodetransport 包内的默认常量。
type NodeTransport struct {
	// StreamQueueFrames 是每条流在内存里缓冲的响应帧数上限；超过即判慢消费。
	StreamQueueFrames int `mapstructure:"stream_queue_frames"`
	// ConsumerWaitTimeout 是消费队列满时等待消费端追上的宽限（如 "10s"）；
	// 0 表示不等待、立即判慢消费（向后兼容旧行为）。
	ConsumerWaitTimeout time.Duration `mapstructure:"consumer_wait_timeout"`
	// BusyWaitTimeout 是节点并发满时短暂排队等待的时长（如 "200ms"）；
	// 0 表示不等待、立即返回 ErrNodeBusy。
	BusyWaitTimeout time.Duration `mapstructure:"busy_wait_timeout"`
	// MaxRequestBytes 是单请求体上限（字节，默认 16MiB）。
	MaxRequestBytes int `mapstructure:"max_request_bytes"`
}

type External struct {
	LLMPriceURL  string `mapstructure:"llm_price_url"`
	UpdateURL    string `mapstructure:"update_url"`
	UpdateAPIURL string `mapstructure:"update_api_url"`
}

type Security struct {
	EncryptionKey string `mapstructure:"encryption_key"`
}

// Cache 配置可选的缓存与状态存储后端。Type 为空（默认）时保持现有
// 内存 + 数据库策略不变（零破坏性变更）；设为 "redis" 时启用 Redis 后端，
// 将统计/运行时状态/限流冷却/失败提示/频道延迟等卸载到 Redis，支持多实例共享。
// 与 database 字段平级，配置方式见 issue #123。
type Cache struct {
	Type  string      `mapstructure:"type"` // "" | "redis"（空=内存，向后兼容）
	Redis RedisConfig `mapstructure:"redis"`
}

// RedisConfig 描述 Redis 单机连接参数。哨兵/集群模式留待后续迭代。
type RedisConfig struct {
	Addr        string        `mapstructure:"addr"`         // "127.0.0.1:6379"
	Password    string        `mapstructure:"password"`     // 可选
	Username    string        `mapstructure:"username"`     // ACL 用户名（可选）
	DB          int           `mapstructure:"db"`           // 0-15
	PoolSize    int           `mapstructure:"pool_size"`    // 0=go-redis 默认（GOMAXPROCS*10）
	DialTimeout time.Duration `mapstructure:"dial_timeout"` // 0=go-redis 默认 5s
	ReadTimeout time.Duration `mapstructure:"read_timeout"` // 0=go-redis 默认 3s
}

type Config struct {
	Server   Server   `mapstructure:"server"`
	Log      Log      `mapstructure:"log"`
	Database Database `mapstructure:"database"`
	Auth     Auth     `mapstructure:"auth"`
	Relay    Relay    `mapstructure:"relay"`
	NodeTransport NodeTransport `mapstructure:"node_transport"`
	External External `mapstructure:"external"`
	Security Security `mapstructure:"security"`
	Cache    Cache    `mapstructure:"cache"`
}

var AppConfig Config

// ephemeralJWTSecret marks whether the JWT secret was generated ephemerally
// during Load() (because it was empty or a known placeholder). When true the
// secret is NOT safe to derive an AES encryption key from — using it would
// cause encrypted data to become unrecoverable after the next restart.
var ephemeralJWTSecret bool

// IsEphemeralJWTSecret reports whether the current JWT secret was generated
// ephemerally this process (i.e. not persisted to config). Callers that need a
// durable key (such as crypto.Init) should refuse to use it.
func IsEphemeralJWTSecret() bool { return ephemeralJWTSecret }

func Load(path string) error {
	configFile := path
	if path != "" {
		viper.SetConfigFile(path)
	} else {
		viper.SetConfigName("config")
		viper.SetConfigType("json")
		viper.AddConfigPath(defaultDataDir())
		configFile = defaultConfigPath()
	}

	viper.AutomaticEnv()
	viper.SetEnvPrefix(APP_NAME)
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	setDefaults()

	if err := viper.ReadInConfig(); err == nil {
		log.Infof("Using config file: %s", viper.ConfigFileUsed())
	} else {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Infof("Config file not found, creating default config")
			if err := os.MkdirAll(filepath.Dir(configFile), 0755); err != nil {
				return wrapConfigPathError("failed to create config directory", filepath.Dir(configFile), err)
			}
			if err := viper.SafeWriteConfigAs(configFile); err != nil {
				return wrapConfigPathError("failed to create default config", configFile, err)
			}
		} else {
			return fmt.Errorf("error reading config file: %w", err)
		}
	}

	if err := viper.Unmarshal(&AppConfig); err != nil {
		return fmt.Errorf("unable to decode config into struct: %w", err)
	}
	if AppConfig.Auth.JWTSecret == "" {
		secret, err := generateJWTSecret()
		if err != nil {
			return fmt.Errorf("failed to generate JWT secret: %w", err)
		}
		AppConfig.Auth.JWTSecret = secret
		ephemeralJWTSecret = true
		log.Warnf("auth.jwt_secret is empty, generated an ephemeral secret for this process; configure OCTOPUS_AUTH_JWT_SECRET or auth.jwt_secret to keep tokens valid across restarts")
	} else if isKnownPlaceholderJWTSecret(AppConfig.Auth.JWTSecret) {
		secret, err := generateJWTSecret()
		if err != nil {
			return fmt.Errorf("failed to generate JWT secret: %w", err)
		}
		AppConfig.Auth.JWTSecret = secret
		ephemeralJWTSecret = true
		log.Warnf("auth.jwt_secret is a known placeholder value; generated an ephemeral secret instead. Set a unique value to keep tokens valid across restarts")
	}
	return nil
}

func SaveDatabaseConfig(dbType, path string) error {
	dbType = strings.TrimSpace(dbType)
	path = strings.TrimSpace(path)
	if dbType == "" || path == "" {
		return fmt.Errorf("database type and path are required")
	}
	viper.Set("database.type", dbType)
	viper.Set("database.path", path)
	if err := viper.WriteConfig(); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	AppConfig.Database.Type = dbType
	AppConfig.Database.Path = path
	return nil
}

// SaveCacheConfig 将缓存后端配置（Redis）写回 config.json。
// 镜像 SaveDatabaseConfig 模式：viper.Set 各字段后 WriteConfig 落盘，再同步内存 AppConfig。
// cacheType 为空表示使用内存后端（关闭 Redis），与未配置时行为一致。
// 因为 Redis 启用是启动时决策（cmd/start.go 仅在 boot 时读 AppConfig.Cache），
// 保存后需重启生效——调用方应提示用户重启（与数据库迁移一致）。
func SaveCacheConfig(cacheType string, redis RedisConfig) error {
	cacheType = strings.TrimSpace(cacheType)
	viper.Set("cache.type", cacheType)
	viper.Set("cache.redis.addr", redis.Addr)
	viper.Set("cache.redis.password", redis.Password)
	viper.Set("cache.redis.username", redis.Username)
	viper.Set("cache.redis.db", redis.DB)
	viper.Set("cache.redis.pool_size", redis.PoolSize)
	// Duration 写为可读字符串（"3s"），与 setDefaults 默认值格式一致。
	if redis.DialTimeout > 0 {
		viper.Set("cache.redis.dial_timeout", redis.DialTimeout.String())
	} else {
		viper.Set("cache.redis.dial_timeout", "")
	}
	if redis.ReadTimeout > 0 {
		viper.Set("cache.redis.read_timeout", redis.ReadTimeout.String())
	} else {
		viper.Set("cache.redis.read_timeout", "")
	}
	if err := viper.WriteConfig(); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	AppConfig.Cache.Type = cacheType
	AppConfig.Cache.Redis = redis
	return nil
}

func setDefaults() {
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.external_url", "")
	viper.SetDefault("database.type", "sqlite")
	viper.SetDefault("database.path", defaultDatabasePath())
	// 日志库默认留空：留空表示与主库共用连接（向后兼容）。
	viper.SetDefault("database.log_type", "")
	viper.SetDefault("database.log_path", "")
	// SQLite per-connection PRAGMA 调优（见 issue #97：低内存环境持续高磁盘 IO）。
	// cache_size 默认 -20000KB（≈20MB），与 internal/db.DefaultSQLiteCacheSize 对齐；
	// mmap_size 默认 0（禁用 mmap，避免物理内存 < 库大小时空洞缺页导致的持续读盘）。
	viper.SetDefault("database.sqlite.cache_size", -20000)
	viper.SetDefault("database.sqlite.mmap_size", int64(0))
	viper.SetDefault("log.level", "info")
	viper.SetDefault("auth.jwt_secret", "")
	viper.SetDefault("relay.max_json_body_bytes", int64(64<<20))
	viper.SetDefault("relay.max_multipart_body_bytes", int64(64<<20))
	// 节点反向传输默认沿用 nodetransport 包内常量（0 = 用代码默认）。
	viper.SetDefault("node_transport.stream_queue_frames", 0)
	viper.SetDefault("node_transport.consumer_wait_timeout", "0s")
	viper.SetDefault("external.llm_price_url", "https://models.dev/api.json")
	viper.SetDefault("external.update_url", "https://github.com/lingyuins/octopus/releases/latest/download")
	viper.SetDefault("external.update_api_url", "https://api.github.com/repos/lingyuins/octopus/releases/latest")
	viper.SetDefault("security.encryption_key", "")
	// 缓存/状态后端默认留空：留空表示沿用内存 + 数据库策略（向后兼容）。
	// 配置 "redis" 时启用 Redis 后端（见 issue #123）。
	viper.SetDefault("cache.type", "")
	// Redis 连接超时默认 3s（issue #135）：比 go-redis 默认 DialTimeout 5s 更激进，
	// 避免远程 Redis 重启期间 TCP hang 导致启动期长时间无日志阻塞。
	viper.SetDefault("cache.redis.dial_timeout", "3s")
	viper.SetDefault("cache.redis.read_timeout", "3s")
}

func defaultDataDir() string {
	if path := strings.TrimSpace(os.Getenv(strings.ToUpper(APP_NAME) + "_DATA_DIR")); path != "" {
		return filepath.Clean(path)
	}
	return "data"
}

// DataDir returns the resolved data directory (from OCTOPUS_DATA_DIR env or
// "data" fallback). Exported so other packages can constrain file operations
// (e.g. SQLite migration paths) to within the data directory.
func DataDir() string {
	return defaultDataDir()
}

func defaultConfigPath() string {
	return filepath.Join(defaultDataDir(), "config.json")
}

func defaultDatabasePath() string {
	return filepath.Join(defaultDataDir(), "data.db")
}

func wrapConfigPathError(action, path string, err error) error {
	if err == nil {
		return nil
	}
	if os.IsPermission(err) {
		return fmt.Errorf("%s %q: %w; make sure the target directory is writable by the current process (the official Docker image runs as UID/GID 1000 and needs write access to /app/data)", action, path, err)
	}
	return fmt.Errorf("%s %q: %w", action, path, err)
}

var knownPlaceholderSecrets = []string{
	"change-this-to-a-long-random-secret",
}

func isKnownPlaceholderJWTSecret(secret string) bool {
	for _, p := range knownPlaceholderSecrets {
		if secret == p {
			return true
		}
	}
	return false
}

func generateJWTSecret() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
