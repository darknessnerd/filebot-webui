package config

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
	"webui-skeleton/internal/logger"

	"github.com/joho/godotenv"
)

type Config struct {
	// Server configuration
	Server ServerConfig `json:"server"`

	// Database configuration
	Database DatabaseConfig `json:"database"`

	// Authentication configuration
	Auth AuthConfig `json:"auth"`

	// Logging configuration
	Debug    bool   `json:"debug"`
	LogLevel string `json:"log_level"`

	// FileBot-WebUI specific
	DirectoryPresets map[string]string `json:"directory_presets"`
}

type ServerConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type DatabaseConfig struct {
	Type     DatabaseType `json:"type"`
	Host     string       `json:"host"`
	Port     int          `json:"port"`
	Username string       `json:"username"`
	Password string       `json:"password"`
	Database string       `json:"database"`
	SSLMode  string       `json:"ssl_mode"`

	// Connection pool settings
	MaxOpenConns    int           `json:"max_open_conns"`
	MaxIdleConns    int           `json:"max_idle_conns"`
	ConnMaxLifetime time.Duration `json:"conn_max_lifetime"`
}

// GetDriverName returns the database driver name based on the database type
func (d DatabaseConfig) GetDriverName() string {
	switch d.Type {
	case PostgreSQL:
		return "postgres"
	case SQLite:
		return "sqlite3"
	default:
		return "sqlite3"
	}
}

// GetDSN returns the data source name for the database connection
func (d DatabaseConfig) GetDSN() string {
	switch d.Type {
	case PostgreSQL:
		return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			d.Host, d.Port, d.Username, d.Password, d.Database, d.SSLMode)
	case SQLite:
		return d.Database
	default:
		return d.Database
	}
}

type AuthProvider string

const (
	AuthProviderPlex AuthProvider = "plex"
)

type AuthConfig struct {
	// JWT Configuration
	JWTSecret    string        `json:"jwt_secret"`
	JWTExpiresIn time.Duration `json:"jwt_expires_in"`
	JWTIssuer    string        `json:"jwt_issuer"`

	// Plex OAuth2 Configuration
	PlexClientID     string `json:"plex_client_id"`
	PlexClientSecret string `json:"plex_client_secret"`
	PlexRedirectURL  string `json:"plex_redirect_url"`
	EnablePlexAuth   bool   `json:"enable_plex_auth"`

	// Session Configuration
	SessionSecret string `json:"session_secret"`

	// Auth Settings
	RequireAuth  bool         `json:"require_auth"`
	AuthProvider AuthProvider `json:"auth_provider"`
}

// DatabaseType represents the type of database
type DatabaseType string

const (
	SQLite     DatabaseType = "sqlite"
	PostgreSQL DatabaseType = "postgresql"
)

// LoadConfiguration loads and validates the application configuration
func LoadConfiguration() (*Config, error) {
	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		logger.Log.Warn().Err(err).Msg(".env file not found or couldn't be loaded")
	}

	config := &Config{}

	// Parse command line flags
	parseFlags(config)

	// Load environment variables and apply defaults
	loadEnvironmentVariables(config)

	// Validate configuration
	if err := validateConfig(config); err != nil {
		logger.Log.Error().Err(err).Msg("Configuration validation failed")
		return nil, err
	}

	return config, nil
}

func parseFlags(config *Config) {
	flag.StringVar(&config.Server.Host, "host", "", "Server host")
	flag.IntVar(&config.Server.Port, "port", 0, "Server port")
	flag.BoolVar(&config.Debug, "debug", true, "Enable debug mode")
	flag.StringVar(&config.LogLevel, "log-level", "debug", "Set log level (debug, info, warn, error)")
	flag.Parse()
}

func loadEnvironmentVariables(config *Config) error {
	// Server configuration
	if config.Server.Host == "" {
		config.Server.Host = getEnvOrDefault("SERVER_HOST", "0.0.0.0")
	}
	if config.Server.Port == 0 {
		config.Server.Port = getEnvAsIntOrDefault("SERVER_PORT", 8080)
	}

	// Database configuration
	config.Database.Type = DatabaseType(getEnvOrDefault("DB_TYPE", "sqlite"))
	config.Database.Host = getEnvOrDefault("DB_HOST", "localhost")
	config.Database.Port = getEnvAsIntOrDefault("DB_PORT", 5432)
	config.Database.Username = getEnvOrDefault("DB_USERNAME", "")
	config.Database.Password = getEnvOrDefault("DB_PASSWORD", "")
	config.Database.Database = getEnvOrDefault("DB_DATABASE", "app.db")
	config.Database.SSLMode = getEnvOrDefault("DB_SSL_MODE", "disable")

	// Database connection pool
	config.Database.MaxOpenConns = getEnvAsIntOrDefault("DB_MAX_OPEN_CONNS", 25)
	config.Database.MaxIdleConns = getEnvAsIntOrDefault("DB_MAX_IDLE_CONNS", 5)
	config.Database.ConnMaxLifetime = getEnvAsDurationOrDefault("DB_CONN_MAX_LIFETIME", 5*time.Minute)

	// Authentication configuration
	config.Auth.JWTSecret = getEnvOrDefault("JWT_SECRET", "your-secret-key")
	config.Auth.JWTExpiresIn = getEnvAsDurationOrDefault("JWT_EXPIRES_IN", 24*time.Hour)
	config.Auth.JWTIssuer = getEnvOrDefault("JWT_ISSUER", "webui-skeleton")
	config.Auth.PlexClientID = getEnvOrDefault("PLEX_CLIENT_ID", "")
	config.Auth.PlexClientSecret = getEnvOrDefault("PLEX_CLIENT_SECRET", "")
	config.Auth.PlexRedirectURL = getEnvOrDefault("PLEX_REDIRECT_URL", "")
	config.Auth.EnablePlexAuth = getEnvAsBoolOrDefault("ENABLE_PLEX_AUTH", true)
	config.Auth.SessionSecret = getEnvOrDefault("SESSION_SECRET", "your-session-secret")
	config.Auth.AuthProvider = AuthProvider(getEnvOrDefault("AUTH_PROVIDER", "both"))

	// Logging configuration
	if !config.Debug {
		config.Debug = getEnvAsBoolOrDefault("DEBUG", true)
	}
	if config.LogLevel == "" {
		config.LogLevel = getEnvOrDefault("LOG_LEVEL", "debug")
	}

	// Directory presets
	presetsRaw := getEnvOrDefault("DIRECTORY_PRESETS", "")
	if presetsRaw == "" {
		return fmt.Errorf("DIRECTORY_PRESETS environment variable must be set. Example: DIRECTORY_PRESETS=Downloads:/downloads,Media Library:/media")
	}
	presets := map[string]string{}
	for _, entry := range strings.Split(presetsRaw, ",") {
		parts := strings.SplitN(entry, ":", 2)
		if len(parts) == 2 {
			presets[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	config.DirectoryPresets = presets
	return nil
}

func validateConfig(config *Config) error {
	if config.Server.Port <= 0 || config.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", config.Server.Port)
	}

	if config.Auth.JWTSecret == "" || config.Auth.JWTSecret == "your-secret-key" {
		return fmt.Errorf("JWT secret must be set when authentication is required")
	}

	// Only require Plex credentials if Plex is enabled
	if config.Auth.AuthProvider == AuthProviderPlex && config.Auth.EnablePlexAuth {
		if config.Auth.PlexClientID == "" || config.Auth.PlexClientSecret == "" {
			return fmt.Errorf("Plex OAuth credentials must be set when Plex authentication is enabled or required")
		}
	}

	return nil
}

// Helper functions
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvAsBoolOrDefault(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

func getEnvAsDurationOrDefault(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

func getEnvAsSliceOrDefault(key string, defaultValue []string, separator string) []string {
	if value := os.Getenv(key); value != "" {
		return strings.Split(value, separator)
	}
	return defaultValue
}
