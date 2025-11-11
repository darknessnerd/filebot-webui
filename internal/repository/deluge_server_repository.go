package repository

import (
	"database/sql"
	"fmt"
	"time"
	"webui-skeleton/internal/models"
)

type DelugeServerRepository struct {
	db *sql.DB
}

func NewDelugeServerRepository(db *sql.DB) *DelugeServerRepository {
	return &DelugeServerRepository{db: db}
}

func (r *DelugeServerRepository) UpsertDelugeServer(user *models.User, server *models.DelugeServer) error {
	// Set timestamps if not already set
	if server.CreatedAt.IsZero() {
		server.CreatedAt = time.Now()
	}
	server.LastSeenAt = time.Now()

	// Set UserID from user
	server.UserID = user.ID

	// Calculate URI if not provided
	if server.URI == "" {
		server.URI = fmt.Sprintf("%s://%s:%d", server.Protocol, server.Host, server.Port)
	}

	query := `INSERT INTO deluge_servers (
		user_id, name, host, port, username, password, protocol, created_at, last_seen_at, 
		preferred, uri, api_version, client_id, session_id, connected, last_error, last_error_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(user_id, name) DO UPDATE SET
		host=excluded.host,
		port=excluded.port,
		username=excluded.username,
		password=excluded.password,
		protocol=excluded.protocol,
		last_seen_at=excluded.last_seen_at,
		uri=excluded.uri,
		api_version=excluded.api_version,
		client_id=excluded.client_id,
		session_id=excluded.session_id,
		connected=excluded.connected,
		last_error=excluded.last_error,
		last_error_at=excluded.last_error_at`

	_, err := r.db.Exec(query,
		server.UserID, server.Name, server.Host, server.Port, server.Username, server.Password, 
		server.Protocol, server.CreatedAt, server.LastSeenAt, server.Preferred, server.URI,
		server.APIVersion, server.ClientID, server.SessionID, server.Connected, 
		server.LastError, server.LastErrorAt,
	)
	return err
}

func (r *DelugeServerRepository) SetPreferredDelugeServer(userID int, serverID int) error {
	// First, set all servers for this user to not preferred
	_, err := r.db.Exec("UPDATE deluge_servers SET preferred=0 WHERE user_id=?", userID)
	if err != nil {
		return err
	}

	// Then set the specified server as preferred
	result, err := r.db.Exec("UPDATE deluge_servers SET preferred=1 WHERE user_id=? AND id=?", userID, serverID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *DelugeServerRepository) GetPreferredDelugeServer(userID int) (*models.DelugeServer, error) {
	row := r.db.QueryRow(`
		SELECT id, user_id, name, host, port, username, password, protocol, created_at, last_seen_at, 
		preferred, uri, api_version, client_id, session_id, connected, last_error, last_error_at 
		FROM deluge_servers WHERE user_id=? AND preferred=1 LIMIT 1`, userID)

	var server models.DelugeServer
	err := row.Scan(
		&server.ID, &server.UserID, &server.Name, &server.Host, &server.Port, &server.Username, 
		&server.Password, &server.Protocol, &server.CreatedAt, &server.LastSeenAt, &server.Preferred, 
		&server.URI, &server.APIVersion, &server.ClientID, &server.SessionID, &server.Connected, 
		&server.LastError, &server.LastErrorAt,
	)
	if err != nil {
		return nil, err
	}

	return &server, nil
}

func (r *DelugeServerRepository) GetServersByUser(userID int) ([]models.DelugeServer, error) {
	rows, err := r.db.Query(`
		SELECT id, user_id, name, host, port, username, password, protocol, created_at, last_seen_at, 
		preferred, uri, api_version, client_id, session_id, connected, last_error, last_error_at 
		FROM deluge_servers WHERE user_id=?`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var servers []models.DelugeServer
	for rows.Next() {
		var server models.DelugeServer
		err := rows.Scan(
			&server.ID, &server.UserID, &server.Name, &server.Host, &server.Port, &server.Username, 
			&server.Password, &server.Protocol, &server.CreatedAt, &server.LastSeenAt, &server.Preferred, 
			&server.URI, &server.APIVersion, &server.ClientID, &server.SessionID, &server.Connected, 
			&server.LastError, &server.LastErrorAt,
		)
		if err != nil {
			return nil, err
		}
		servers = append(servers, server)
	}

	return servers, nil
}

func (r *DelugeServerRepository) DeleteServer(serverID int, userID int) error {
	result, err := r.db.Exec("DELETE FROM deluge_servers WHERE id=? AND user_id=?", serverID, userID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *DelugeServerRepository) UpdateServerStatus(serverID int, connected bool, lastError string) error {
	var lastErrorAt time.Time
	if lastError != "" {
		lastErrorAt = time.Now()
	}

	_, err := r.db.Exec(
		"UPDATE deluge_servers SET connected=?, last_error=?, last_error_at=? WHERE id=?",
		connected, lastError, lastErrorAt, serverID,
	)
	return err
}

// Interface for the repository
type DelugeServerRepositoryInterface interface {
	UpsertDelugeServer(user *models.User, server *models.DelugeServer) error
	SetPreferredDelugeServer(userID int, serverID int) error
	GetPreferredDelugeServer(userID int) (*models.DelugeServer, error)
	GetServersByUser(userID int) ([]models.DelugeServer, error)
	DeleteServer(serverID int, userID int) error
	UpdateServerStatus(serverID int, connected bool, lastError string) error
}
