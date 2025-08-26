package repository

import (
	"database/sql"
	"webui-skeleton/internal/logger"
	"webui-skeleton/internal/models"
)

type PlexServerRepository struct {
	db *sql.DB
}

func NewPlexServerRepository(db *sql.DB) *PlexServerRepository {
	return &PlexServerRepository{db: db}
}

func (r *PlexServerRepository) UpsertPlexServer(user *models.User, server *models.PlexServer) error {
	query := `INSERT INTO plex_servers (
		user_id, name, product, product_version, client_identifier, created_at, last_seen_at, provides, public_address, access_token, owned, home, synced, relay, presence, https_required, preferred, platform, platform_version, device, owner_id, source_title, public_address_matches, dns_rebinding_protection, nat_loopback_supported
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(user_id, name) DO UPDATE SET
		product=excluded.product,
		product_version=excluded.product_version,
		client_identifier=excluded.client_identifier,
		created_at=excluded.created_at,
		last_seen_at=excluded.last_seen_at,
		provides=excluded.provides,
		public_address=excluded.public_address,
		access_token=excluded.access_token,
		owned=excluded.owned,
		home=excluded.home,
		synced=excluded.synced,
		relay=excluded.relay,
		presence=excluded.presence,
		https_required=excluded.https_required,
		platform=excluded.platform,
		platform_version=excluded.platform_version,
		device=excluded.device,
		owner_id=excluded.owner_id,
		source_title=excluded.source_title,
		public_address_matches=excluded.public_address_matches,
		dns_rebinding_protection=excluded.dns_rebinding_protection,
		nat_loopback_supported=excluded.nat_loopback_supported`
	result, err := r.db.Exec(query,
		user.ID, server.Name, server.Product, server.ProductVersion, server.ClientIdentifier, server.CreatedAt, server.LastSeenAt, server.Provides, server.PublicAddress, server.AccessToken, server.Owned, server.Home, server.Synced, server.Relay, server.Presence, server.HttpsRequired, server.Preferred, server.Platform, server.PlatformVersion, server.Device, server.OwnerID, server.SourceTitle, server.PublicAddressMatches, server.DNSRebindingProtection, server.NATLoopbackSupported,
	)
	if err != nil {
		return err
	}
	var serverID int64
	if id, err := result.LastInsertId(); err == nil && id > 0 {
		serverID = id
	} else {
		row := r.db.QueryRow("SELECT id FROM plex_servers WHERE user_id=? AND name=? LIMIT 1", user.ID, server.Name)
		_ = row.Scan(&serverID)
	}
	for _, conn := range server.Connections {
		conn.ServerID = int(serverID)
		err := r.UpsertPlexServerConnection(&conn)
		if err != nil {
			logger.Log.Error().Err(err).Msg("Failed to upsert Plex server connection")
		}
	}
	return nil
}

func (r *PlexServerRepository) UpsertPlexServerConnection(conn *models.PlexServerConnection) error {
	// Check for existing connection before insert
	row := r.db.QueryRow("SELECT id FROM plex_server_connections WHERE server_id=? AND uri=? LIMIT 1", conn.ServerID, conn.URI)
	var existingID int
	err := row.Scan(&existingID)
	if err == sql.ErrNoRows {
		// Insert new connection
		query := `INSERT INTO plex_server_connections (
			server_id, protocol, address, port, uri, local, relay, ipv6
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
		_, err := r.db.Exec(query,
			conn.ServerID, conn.Protocol, conn.Address, conn.Port, conn.URI, conn.Local, conn.Relay, conn.IPv6,
		)
		return err
	} else if err == nil {
		// Update existing connection
		query := `UPDATE plex_server_connections SET
			protocol=?, address=?, port=?, local=?, relay=?, ipv6=?
			WHERE id=?`
		_, err := r.db.Exec(query,
			conn.Protocol, conn.Address, conn.Port, conn.Local, conn.Relay, conn.IPv6, existingID,
		)
		return err
	}
	return err
}

func (r *PlexServerRepository) SetPreferredPlexServer(userID int, serverID int) error {
	_, err := r.db.Exec("UPDATE plex_servers SET preferred=0 WHERE user_id=?", userID)
	if err != nil {
		return err
	}
	result, err := r.db.Exec("UPDATE plex_servers SET preferred=1 WHERE user_id=? AND id=?", userID, serverID)
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

func (r *PlexServerRepository) GetPreferredPlexServer(userID int) (*models.PlexServer, error) {
	row := r.db.QueryRow("SELECT id, user_id, name, product, product_version, client_identifier, created_at, last_seen_at, provides, public_address, access_token, owned, home, synced, relay, presence, https_required, preferred, platform, platform_version, device, owner_id, source_title, public_address_matches, dns_rebinding_protection, nat_loopback_supported FROM plex_servers WHERE user_id=? AND preferred=1 LIMIT 1", userID)
	var server models.PlexServer
	err := row.Scan(&server.ID, &server.UserID, &server.Name, &server.Product, &server.ProductVersion, &server.ClientIdentifier, &server.CreatedAt, &server.LastSeenAt, &server.Provides, &server.PublicAddress, &server.AccessToken, &server.Owned, &server.Home, &server.Synced, &server.Relay, &server.Presence, &server.HttpsRequired, &server.Preferred, &server.Platform, &server.PlatformVersion, &server.Device, &server.OwnerID, &server.SourceTitle, &server.PublicAddressMatches, &server.DNSRebindingProtection, &server.NATLoopbackSupported)
	if err != nil {
		return nil, err
	}
	// Fetch connections for this server
	connRows, err := r.db.Query("SELECT id, server_id, protocol, address, port, uri, local, relay, ipv6 FROM plex_server_connections WHERE server_id=?", server.ID)
	if err == nil {
		var connections []models.PlexServerConnection
		for connRows.Next() {
			var conn models.PlexServerConnection
			err := connRows.Scan(&conn.ID, &conn.ServerID, &conn.Protocol, &conn.Address, &conn.Port, &conn.URI, &conn.Local, &conn.Relay, &conn.IPv6)
			if err == nil {
				connections = append(connections, conn)
			}
		}
		connRows.Close()
		server.Connections = connections
	}
	return &server, nil
}

func (r *PlexServerRepository) BatchUpsertAndFetchServers(user *models.User, servers []models.PlexServer) ([]models.PlexServer, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return nil, err
	}
	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		}
	}()

	var connectionQueue []models.PlexServerConnection
	for _, server := range servers {
		logger.Log.Debug().
			Str("user", user.Name).
			Int("user_id", user.ID).
			Str("server_name", server.Name).
			Str("client_identifier", server.ClientIdentifier).
			Msg("Attempting upsert for server")
		query := `INSERT INTO plex_servers (
			user_id, name, product, product_version, client_identifier, created_at, last_seen_at, provides, public_address, access_token, owned, home, synced, relay, presence, https_required, preferred, platform, platform_version, device, owner_id, source_title, public_address_matches, dns_rebinding_protection, nat_loopback_supported
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id, name) DO UPDATE SET
			product=excluded.product,
			product_version=excluded.product_version,
			client_identifier=excluded.client_identifier,
			created_at=excluded.created_at,
			last_seen_at=excluded.last_seen_at,
			provides=excluded.provides,
			public_address=excluded.public_address,
			access_token=excluded.access_token,
			owned=excluded.owned,
			home=excluded.home,
			synced=excluded.synced,
			relay=excluded.relay,
			presence=excluded.presence,
			https_required=excluded.https_required,
			platform=excluded.platform,
			platform_version=excluded.platform_version,
			device=excluded.device,
			owner_id=excluded.owner_id,
			source_title=excluded.source_title,
			public_address_matches=excluded.public_address_matches,
			dns_rebinding_protection=excluded.dns_rebinding_protection,
			nat_loopback_supported=excluded.nat_loopback_supported`
		result, err := tx.Exec(query,
			user.ID, server.Name, server.Product, server.ProductVersion, server.ClientIdentifier, server.CreatedAt, server.LastSeenAt, server.Provides, server.PublicAddress, server.AccessToken, server.Owned, server.Home, server.Synced, server.Relay, server.Presence, server.HttpsRequired, server.Preferred, server.Platform, server.PlatformVersion, server.Device, server.OwnerID, server.SourceTitle, server.PublicAddressMatches, server.DNSRebindingProtection, server.NATLoopbackSupported,
		)
		if err != nil {
			tx.Rollback()
			logger.Log.Error().
				Str("user", user.Name).
				Int("user_id", user.ID).
				Str("server_name", server.Name).
				Str("client_identifier", server.ClientIdentifier).
				Err(err).
				Msgf("Failed to upsert server for user")
			return nil, err
		}
		var serverID int64
		if id, err := result.LastInsertId(); err == nil && id > 0 {
			serverID = id
		} else {
			row := tx.QueryRow("SELECT id FROM plex_servers WHERE user_id=? AND name=? LIMIT 1", user.ID, server.Name)
			_ = row.Scan(&serverID)
		}
		for _, conn := range server.Connections {
			conn.ServerID = int(serverID)
			connectionQueue = append(connectionQueue, conn)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	// Upsert connections after transaction to avoid database lock
	for _, conn := range connectionQueue {
		err := r.UpsertPlexServerConnection(&conn)
		if err != nil {
			logger.Log.Error().Err(err).Msg("Failed to upsert Plex server connection (batch)")
		}
	}

	return r.GetServersByUser(user.ID)
}

func (r *PlexServerRepository) GetServersByUser(userID int) ([]models.PlexServer, error) {
	rows, err := r.db.Query("SELECT id, user_id, name, product, product_version, client_identifier, created_at, last_seen_at, provides, public_address, access_token, owned, home, synced, relay, presence, https_required, preferred, platform, platform_version, device, owner_id, source_title, public_address_matches, dns_rebinding_protection, nat_loopback_supported FROM plex_servers WHERE user_id=?", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var servers []models.PlexServer
	for rows.Next() {
		var server models.PlexServer
		err := rows.Scan(&server.ID, &server.UserID, &server.Name, &server.Product, &server.ProductVersion, &server.ClientIdentifier, &server.CreatedAt, &server.LastSeenAt, &server.Provides, &server.PublicAddress, &server.AccessToken, &server.Owned, &server.Home, &server.Synced, &server.Relay, &server.Presence, &server.HttpsRequired, &server.Preferred, &server.Platform, &server.PlatformVersion, &server.Device, &server.OwnerID, &server.SourceTitle, &server.PublicAddressMatches, &server.DNSRebindingProtection, &server.NATLoopbackSupported)
		if err != nil {
			return nil, err
		}
		// Fetch connections for this server
		connRows, err := r.db.Query("SELECT id, server_id, protocol, address, port, uri, local, relay, ipv6 FROM plex_server_connections WHERE server_id=?", server.ID)
		if err == nil {
			var connections []models.PlexServerConnection
			for connRows.Next() {
				var conn models.PlexServerConnection
				err := connRows.Scan(&conn.ID, &conn.ServerID, &conn.Protocol, &conn.Address, &conn.Port, &conn.URI, &conn.Local, &conn.Relay, &conn.IPv6)
				if err == nil {
					connections = append(connections, conn)
				}
			}
			connRows.Close()
			server.Connections = connections
		}
		servers = append(servers, server)
	}
	return servers, nil
}
