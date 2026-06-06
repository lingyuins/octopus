package conf

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lingyuins/octopus/internal/utils/log"
	"github.com/spf13/viper"
)

type Server struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

type Log struct {
	Level string `mapstructure:"level"`
}

type Database struct {
	Type string `mapstructure:"type"`
	Path string `mapstructure:"path"`
}

type Auth struct {
	JWTSecret string `mapstructure:"jwt_secret"`
}

type Relay struct {
	MaxJSONBodyBytes      int64 `mapstructure:"max_json_body_bytes"`
	MaxMultipartBodyBytes int64 `mapstructure:"max_multipart_body_bytes"`
}

type External struct {
	LLMPriceURL  string `mapstructure:"llm_price_url"`
	UpdateURL    string `mapstructure:"update_url"`
	UpdateAPIURL string `mapstructure:"update_api_url"`
}

type Security struct {
	EncryptionKey string `mapstructure:"encryption_key"`
}

type Config struct {
	Server   Server   `mapstructure:"server"`
	Log      Log      `mapstructure:"log"`
	Database Database `mapstructure:"database"`
	Auth     Auth     `mapstructure:"auth"`
	Relay    Relay    `mapstructure:"relay"`
	External External `mapstructure:"external"`
	Security Security `mapstructure:"security"`
}

var AppConfig Config

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
		log.Warnf("auth.jwt_secret is empty, generated an ephemeral secret for this process; configure OCTOPUS_AUTH_JWT_SECRET or auth.jwt_secret to keep tokens valid across restarts")
	} else if isKnownPlaceholderJWTSecret(AppConfig.Auth.JWTSecret) {
		secret, err := generateJWTSecret()
		if err != nil {
			return fmt.Errorf("failed to generate JWT secret: %w", err)
		}
		AppConfig.Auth.JWTSecret = secret
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

func setDefaults() {
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("database.type", "sqlite")
	viper.SetDefault("database.path", defaultDatabasePath())
	viper.SetDefault("log.level", "info")
	viper.SetDefault("auth.jwt_secret", "")
	viper.SetDefault("relay.max_json_body_bytes", int64(64<<20))
	viper.SetDefault("relay.max_multipart_body_bytes", int64(64<<20))
	viper.SetDefault("external.llm_price_url", "https://models.dev/api.json")
	viper.SetDefault("external.update_url", "https://github.com/lingyuins/octopus/releases/latest/download")
	viper.SetDefault("external.update_api_url", "https://api.github.com/repos/lingyuins/octopus/releases/latest")
	viper.SetDefault("security.encryption_key", "")
}

func defaultDataDir() string {
	if path := strings.TrimSpace(os.Getenv(strings.ToUpper(APP_NAME) + "_DATA_DIR")); path != "" {
		return filepath.Clean(path)
	}
	return "data"
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
