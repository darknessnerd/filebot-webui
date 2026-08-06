package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	ServerHost string
	ServerPort string

	JWTSecret    string
	JWTExpiresIn time.Duration
	JWTIssuer    string

	PlexClientID              string
	PlexRedirectURL           string
	TMDBAccessToken           string
	AniDBClient               string
	AniDBClientVer            string
	AniDBProtoVer             string
	AniDBBaseURL              string
	AniDBTitlesFile           string
	AniDBTitlesURL            string
	AniDBRefreshTitlesOnStart bool
	AniDBSchedulerEnabled     bool
	AniDBSchedulerInterval    time.Duration
	AniDBMinFetchInterval     time.Duration

	DelugeHost     string
	DelugePort     string
	DelugeUsername string
	DelugePassword string

	MediaRoot string

	DBType     string
	DBDatabase string
	DBHost     string
	DBPort     string
	DBUsername string
	DBPassword string
	DBSSLMode  string

	LogLevel string
	Debug    bool
	DevMode  bool
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		ServerHost:      getEnv("SERVER_HOST", "0.0.0.0"),
		ServerPort:      getEnv("SERVER_PORT", "8080"),
		JWTIssuer:       getEnv("JWT_ISSUER", "filebot-webui"),
		PlexClientID:    getEnv("PLEX_CLIENT_ID", ""),
		PlexRedirectURL: getEnv("PLEX_REDIRECT_URL", ""),
		TMDBAccessToken: getEnv("TMDB_ACCESS_TOKEN", ""),
		AniDBClient:     getEnv("ANIDB_CLIENT", ""),
		AniDBClientVer:  getEnv("ANIDB_CLIENTVER", ""),
		AniDBProtoVer:   getEnv("ANIDB_PROTOVER", "1"),
		AniDBBaseURL:    getEnv("ANIDB_BASE_URL", "http://api.anidb.net:9001/httpapi"),
		AniDBTitlesFile: getEnv("ANIDB_TITLES_FILE", ""),
		AniDBTitlesURL:  getEnv("ANIDB_TITLES_URL", ""),
		DelugeHost:      getEnv("DELUGE_HOST", ""),
		DelugePort:      getEnv("DELUGE_PORT", "8112"),
		DelugeUsername:  getEnv("DELUGE_USERNAME", ""),
		DelugePassword:  getEnv("DELUGE_PASSWORD", ""),
		DBType:          getEnv("DB_TYPE", "sqlite"),
		DBDatabase:      getEnv("DB_DATABASE", "./data/app.db"),
		DBHost:          getEnv("DB_HOST", "localhost"),
		DBPort:          getEnv("DB_PORT", "5432"),
		DBUsername:      getEnv("DB_USERNAME", ""),
		DBPassword:      getEnv("DB_PASSWORD", ""),
		DBSSLMode:       getEnv("DB_SSL_MODE", "disable"),
		LogLevel:        getEnv("LOG_LEVEL", "info"),
	}

	cfg.Debug, _ = strconv.ParseBool(getEnv("DEBUG", "false"))
	cfg.DevMode, _ = strconv.ParseBool(getEnv("DEV_MODE", "false"))
	cfg.AniDBRefreshTitlesOnStart, _ = strconv.ParseBool(getEnv("ANIDB_REFRESH_TITLES_ON_START", "false"))
	cfg.AniDBSchedulerEnabled, _ = strconv.ParseBool(getEnv("ANIDB_SCHEDULER_ENABLED", "false"))

	siStr := getEnv("ANIDB_SCHEDULER_INTERVAL", "12h")
	schedulerInterval, err := time.ParseDuration(siStr)
	if err != nil {
		return nil, fmt.Errorf("invalid ANIDB_SCHEDULER_INTERVAL %q: %w", siStr, err)
	}
	cfg.AniDBSchedulerInterval = schedulerInterval

	mfStr := getEnv("ANIDB_MIN_FETCH_INTERVAL", "24h")
	minFetchInterval, err := time.ParseDuration(mfStr)
	if err != nil {
		return nil, fmt.Errorf("invalid ANIDB_MIN_FETCH_INTERVAL %q: %w", mfStr, err)
	}
	cfg.AniDBMinFetchInterval = minFetchInterval

	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return nil, errors.New("JWT_SECRET is required")
	}
	cfg.JWTSecret = secret

	mediaRoot := os.Getenv("MEDIA_ROOT")
	if mediaRoot == "" && !cfg.DevMode {
		return nil, errors.New("MEDIA_ROOT is required")
	}
	if mediaRoot == "" {
		mediaRoot = "/dev/null/media"
	}
	cfg.MediaRoot = mediaRoot

	expStr := getEnv("JWT_EXPIRES_IN", "24h")
	exp, err := time.ParseDuration(expStr)
	if err != nil {
		return nil, fmt.Errorf("invalid JWT_EXPIRES_IN %q: %w", expStr, err)
	}
	cfg.JWTExpiresIn = exp

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
