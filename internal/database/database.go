package database

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"webui-skeleton/internal/config"
	"webui-skeleton/internal/logger"
)

type DB struct {
	DB     *sql.DB
	config *config.DatabaseConfig
}

// New creates a new database instance
func New(config *config.DatabaseConfig) *DB {
	return &DB{
		config: config,
	}
}

// Connect establishes a connection to the database
func (db *DB) Connect() error {
	var err error

	logger.Log.Info().
		Str("type", string(db.config.Type)).
		Str("database", db.config.Database).
		Msg("Connecting to database")

	db.DB, err = sql.Open(db.config.GetDriverName(), db.config.GetDSN())
	if err != nil {
		return fmt.Errorf("failed to open database connection: %w", err)
	}

	// Configure connection pool
	db.DB.SetMaxOpenConns(db.config.MaxOpenConns)
	db.DB.SetMaxIdleConns(db.config.MaxIdleConns)
	db.DB.SetConnMaxLifetime(db.config.ConnMaxLifetime)

	// Test the connection
	if err := db.DB.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	if db.config.Type == config.SQLite {
		_, err := db.DB.Exec("PRAGMA journal_mode=WAL;")
		if err != nil {
			return fmt.Errorf("failed to enable WAL mode: %w", err)
		}
		logger.Log.Info().Msg("SQLite WAL mode enabled")
	}

	logger.Log.Info().Msg("✅ Database connection established")
	return nil
}

// Close closes the database connection
func (db *DB) Close() error {
	if db.DB != nil {
		logger.Log.Info().Msg("Closing database connection")
		return db.DB.Close()
	}
	return nil
}

// Migrate runs database migrations
func (db *DB) Migrate() error {
	logger.Log.Info().Msg("Running database migrations")

	// Create users table
	usersSQL := `
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			email VARCHAR(255) UNIQUE,
			name VARCHAR(255),
			picture VARCHAR(500),
			plex_id VARCHAR(255) UNIQUE,
			plex_username VARCHAR(255),
			plex_email VARCHAR(255),
			plex_avatar VARCHAR(500),
			plex_token VARCHAR(500),
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`

	if db.config.Type == config.PostgreSQL {
		usersSQL = `
			CREATE TABLE IF NOT EXISTS users (
				id SERIAL PRIMARY KEY,
				email VARCHAR(255) UNIQUE,
				name VARCHAR(255),
				picture VARCHAR(500),
				plex_id VARCHAR(255) UNIQUE,
				plex_username VARCHAR(255),
				plex_email VARCHAR(255),
				plex_avatar VARCHAR(500),
				plex_token VARCHAR(500),
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			)`
	}

	if _, err := db.DB.Exec(usersSQL); err != nil {
		return fmt.Errorf("failed to create users table: %w", err)
	}

	// Create sessions table for session management
	sessionsSQL := `
		CREATE TABLE IF NOT EXISTS sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			token VARCHAR(500) NOT NULL,
			expires_at DATETIME NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`

	if db.config.Type == config.PostgreSQL {
		sessionsSQL = `
			CREATE TABLE IF NOT EXISTS sessions (
				id SERIAL PRIMARY KEY,
				user_id INTEGER NOT NULL,
				token VARCHAR(500) NOT NULL,
				expires_at TIMESTAMP NOT NULL,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
			)`
	}

	if _, err := db.DB.Exec(sessionsSQL); err != nil {
		return fmt.Errorf("failed to create sessions table: %w", err)
	}

	// Create plex_servers table
	plexServersSQL := `
		CREATE TABLE IF NOT EXISTS plex_servers (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			name VARCHAR(255),
			product VARCHAR(255),
			product_version VARCHAR(255),
			client_identifier VARCHAR(255),
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_seen_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			provides VARCHAR(255),
			public_address VARCHAR(255),
			access_token VARCHAR(500),
			owned BOOLEAN,
			home BOOLEAN,
			synced BOOLEAN,
			relay BOOLEAN,
			presence BOOLEAN,
			https_required BOOLEAN,
			preferred BOOLEAN,
			platform VARCHAR(255),
			platform_version VARCHAR(255),
			device VARCHAR(255),
			owner_id VARCHAR(255),
			source_title VARCHAR(255),
			public_address_matches BOOLEAN,
			dns_rebinding_protection BOOLEAN,
			nat_loopback_supported BOOLEAN,
			movie_format TEXT DEFAULT '{ plex }',
			series_format TEXT DEFAULT '{ plex }',
			anime_format TEXT DEFAULT '{ plex }',
			music_format TEXT DEFAULT '{ plex }',
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			UNIQUE(user_id, name)
		)`

	if db.config.Type == config.PostgreSQL {
		plexServersSQL = `
			CREATE TABLE IF NOT EXISTS plex_servers (
				id SERIAL PRIMARY KEY,
				user_id INTEGER NOT NULL,
				name VARCHAR(255),
				product VARCHAR(255),
				product_version VARCHAR(255),
				client_identifier VARCHAR(255),
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				last_seen_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				provides VARCHAR(255),
				public_address VARCHAR(255),
				access_token VARCHAR(500),
				owned BOOLEAN,
				home BOOLEAN,
				synced BOOLEAN,
				relay BOOLEAN,
				presence BOOLEAN,
				https_required BOOLEAN,
				preferred BOOLEAN,
				platform VARCHAR(255),
				platform_version VARCHAR(255),
				device VARCHAR(255),
				owner_id VARCHAR(255),
				source_title VARCHAR(255),
				public_address_matches BOOLEAN,
				dns_rebinding_protection BOOLEAN,
				nat_loopback_supported BOOLEAN,
				movie_format TEXT DEFAULT '{ plex }',
				series_format TEXT DEFAULT '{ plex }',
				anime_format TEXT DEFAULT '{ plex }',
				music_format TEXT DEFAULT '{ plex }',
				FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
				UNIQUE(user_id, name)
			)`
	}

	if _, err := db.DB.Exec(plexServersSQL); err != nil {
		return fmt.Errorf("failed to create plex_servers table: %w", err)
	}

	// Create plex_server_connections table
	plexServerConnectionsSQL := `
		CREATE TABLE IF NOT EXISTS plex_server_connections (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			server_id INTEGER NOT NULL,
			protocol VARCHAR(50),
			address VARCHAR(255),
			port INTEGER,
			uri VARCHAR(255),
			local BOOLEAN,
			relay BOOLEAN,
			ipv6 BOOLEAN,
			FOREIGN KEY (server_id) REFERENCES plex_servers(id) ON DELETE CASCADE
		)`

	if db.config.Type == config.PostgreSQL {
		plexServerConnectionsSQL = `
			CREATE TABLE IF NOT EXISTS plex_server_connections (
				id SERIAL PRIMARY KEY,
				server_id INTEGER NOT NULL,
				protocol VARCHAR(50),
				address VARCHAR(255),
				port INTEGER,
				uri VARCHAR(255),
				local BOOLEAN,
				relay BOOLEAN,
				ipv6 BOOLEAN,
				FOREIGN KEY (server_id) REFERENCES plex_servers(id) ON DELETE CASCADE
			)`
	}

	if _, err := db.DB.Exec(plexServerConnectionsSQL); err != nil {
		return fmt.Errorf("failed to create plex_server_connections table: %w", err)
	}

	// Create deluge_servers table
	delugeServersSQL := `
		CREATE TABLE IF NOT EXISTS deluge_servers (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			name VARCHAR(255),
			host VARCHAR(255),
			port INTEGER,
			username VARCHAR(255),
			password VARCHAR(255),
			protocol VARCHAR(50),
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_seen_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			preferred BOOLEAN,
			uri VARCHAR(255),
			api_version VARCHAR(50),
			client_id VARCHAR(255),
			session_id VARCHAR(255),
			connected BOOLEAN,
			last_error TEXT,
			last_error_at DATETIME,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			UNIQUE(user_id, name)
		)`

	if db.config.Type == config.PostgreSQL {
		delugeServersSQL = `
			CREATE TABLE IF NOT EXISTS deluge_servers (
				id SERIAL PRIMARY KEY,
				user_id INTEGER NOT NULL,
				name VARCHAR(255),
				host VARCHAR(255),
				port INTEGER,
				username VARCHAR(255),
				password VARCHAR(255),
				protocol VARCHAR(50),
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				last_seen_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				preferred BOOLEAN,
				uri VARCHAR(255),
				api_version VARCHAR(50),
				client_id VARCHAR(255),
				session_id VARCHAR(255),
				connected BOOLEAN,
				last_error TEXT,
				last_error_at TIMESTAMP,
				FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
				UNIQUE(user_id, name)
			)`
	}

	if _, err := db.DB.Exec(delugeServersSQL); err != nil {
		return fmt.Errorf("failed to create deluge_servers table: %w", err)
	}

	logger.Log.Info().Msg("✅ Database migrations completed")
	return nil
}
