package models

import "time"

// PlexServer represents a Plex server resource associated with a user
// See: https://plex.tv/api/v2/resources

type PlexServer struct {
	ID                     int                    `json:"id" db:"id"`
	UserID                 int                    `json:"user_id" db:"user_id"`
	Name                   string                 `json:"name" db:"name"`
	Product                string                 `json:"product" db:"product"`
	ProductVersion         string                 `json:"product_version" db:"product_version"`
	ClientIdentifier       string                 `json:"client_identifier" db:"client_identifier"`
	CreatedAt              time.Time              `json:"created_at" db:"created_at"`
	LastSeenAt             time.Time              `json:"last_seen_at" db:"last_seen_at"`
	Provides               string                 `json:"provides" db:"provides"`
	PublicAddress          string                 `json:"public_address" db:"public_address"`
	AccessToken            string                 `json:"access_token" db:"access_token"`
	Owned                  bool                   `json:"owned" db:"owned"`
	Home                   bool                   `json:"home" db:"home"`
	Synced                 bool                   `json:"synced" db:"synced"`
	Relay                  bool                   `json:"relay" db:"relay"`
	Presence               bool                   `json:"presence" db:"presence"`
	HttpsRequired          bool                   `json:"https_required" db:"https_required"`
	Preferred              bool                   `json:"preferred" db:"preferred"` // User's preferred server
	Platform               string                 `json:"platform" db:"platform"`
	PlatformVersion        string                 `json:"platform_version" db:"platform_version"`
	Device                 string                 `json:"device" db:"device"`
	OwnerID                string                 `json:"owner_id" db:"owner_id"`
	SourceTitle            string                 `json:"source_title" db:"source_title"`
	PublicAddressMatches   bool                   `json:"public_address_matches" db:"public_address_matches"`
	DNSRebindingProtection bool                   `json:"dns_rebinding_protection" db:"dns_rebinding_protection"`
	NATLoopbackSupported   bool                   `json:"nat_loopback_supported" db:"nat_loopback_supported"`
	Connections            []PlexServerConnection `json:"connections" db:"-"`
}

// PlexServerConnection represents a connection for a Plex server
// This can be stored as a separate table or as JSON in the PlexServer table

type PlexServerConnection struct {
	ID       int    `json:"id" db:"id"`
	ServerID int    `json:"server_id" db:"server_id"`
	Protocol string `json:"protocol" db:"protocol"`
	Address  string `json:"address" db:"address"`
	Port     int    `json:"port" db:"port"`
	URI      string `json:"uri" db:"uri"`
	Local    bool   `json:"local" db:"local"`
	Relay    bool   `json:"relay" db:"relay"`
	IPv6     bool   `json:"ipv6" db:"ipv6"`
}
