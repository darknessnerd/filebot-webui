package repository

import (
	"database/sql"
	"fmt"
)

func RunMigrations(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			plex_id       TEXT UNIQUE NOT NULL,
			plex_username TEXT,
			plex_email    TEXT,
			plex_avatar   TEXT,
			plex_token    TEXT,
			created_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at    DATETIME DEFAULT CURRENT_TIMESTAMP
		)`)
	if err != nil {
		return fmt.Errorf("repository.RunMigrations users: %w", err)
	}
	return nil
}
