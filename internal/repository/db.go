package repository

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"

	"github.com/darknessnerd/filebot-webui/internal/config"
)

func Open(cfg *config.Config) (*sql.DB, error) {
	var driver, dsn string

	switch cfg.DBType {
	case "postgresql", "postgres":
		driver = "postgres"
		dsn = fmt.Sprintf(
			"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
			cfg.DBHost, cfg.DBPort, cfg.DBUsername, cfg.DBPassword, cfg.DBDatabase, cfg.DBSSLMode,
		)
	default:
		driver = "sqlite3"
		dsn = cfg.DBDatabase
	}

	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("repository.Open: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("repository.Open ping: %w", err)
	}

	if driver == "sqlite3" {
		if _, err := db.Exec("PRAGMA journal_mode=WAL;"); err != nil {
			db.Close()
			return nil, fmt.Errorf("repository.Open WAL: %w", err)
		}
	}

	return db, nil
}
