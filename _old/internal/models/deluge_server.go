package models

import "time"

// DelugeServer represents a Deluge server configuration associated with a user
type DelugeServer struct {
	ID           int       `json:"id" db:"id"`
	UserID       int       `json:"user_id" db:"user_id"`
	Name         string    `json:"name" db:"name"`
	Host         string    `json:"host" db:"host"`
	Port         int       `json:"port" db:"port"`
	Username     string    `json:"username" db:"username"`
	Password     string    `json:"password" db:"password"`
	Protocol     string    `json:"protocol" db:"protocol"` // http or https
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	LastSeenAt   time.Time `json:"last_seen_at" db:"last_seen_at"`
	Preferred    bool      `json:"preferred" db:"preferred"` // User's preferred server
	URI          string    `json:"uri" db:"uri"`             // Full URI to the server (protocol://host:port)
	APIVersion   string    `json:"api_version" db:"api_version"`
	ClientID     string    `json:"client_id" db:"client_id"` // Deluge client ID for authentication
	SessionID    string    `json:"session_id" db:"session_id"`
	Connected    bool      `json:"connected" db:"connected"`
	LastError    string    `json:"last_error" db:"last_error"`
	LastErrorAt  time.Time `json:"last_error_at" db:"last_error_at"`
}