package plex

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/darknessnerd/filebot-webui/internal/logger"
)

// plexDirectRe matches hostnames like 192-168-178-51.<hash>.plex.direct
// and captures the IPv4 address encoded as dashes.
var plexDirectRe = regexp.MustCompile(`^(\d{1,3}-\d{1,3}-\d{1,3}-\d{1,3})\.[a-f0-9]+\.plex\.direct$`)

// PlexAPIClient defines the interface for Plex API operations
type PlexAPIClient interface {
	RefreshLibraries(ctx context.Context, plexToken string) error
}

// Client represents a Plex API client
type Client struct {
	http         *http.Client
	log          logger.Logger
	resourcesURL string
	dnsResolver  func(ctx context.Context, host string) ([]string, error) // For testing
}

// NewClient creates a new Plex client
func NewClient(log logger.Logger) *Client {
	return &Client{
		http: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					Timeout:   5 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
				TLSHandshakeTimeout:   10 * time.Second,
				ResponseHeaderTimeout: 10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
				MaxIdleConns:          10,
				IdleConnTimeout:       30 * time.Second,
			},
		},
		log:          log,
		resourcesURL: "https://clients.plex.tv/api/v2/resources?includeHttps=1&includeRelay=1&includeIPv6=1",
		dnsResolver:  net.DefaultResolver.LookupHost,
	}
}

// RefreshLibraries refreshes all Plex libraries
func (c *Client) RefreshLibraries(ctx context.Context, plexToken string) error {
	c.log.Debug().Msg("=== Starting Plex library refresh ===")
	c.log.Debug().Str("token_prefix", c.maskToken(plexToken)).Msg("Using Plex token")

	serverURL, serverToken, err := c.resolveServerURL(ctx, plexToken)
	if err != nil {
		return fmt.Errorf("plex.RefreshLibraries resolve: %w", err)
	}
	c.log.Debug().Str("server_url", serverURL).Msg("Server resolved successfully")

	sectionIDs, err := c.listSections(ctx, serverURL, serverToken)
	if err != nil {
		return fmt.Errorf("plex.RefreshLibraries sections: %w", err)
	}
	c.log.Debug().Int("sections", len(sectionIDs)).Msg("Sections listed")

	refreshed := 0
	for _, id := range sectionIDs {
		c.log.Debug().Str("section_id", id).Msg("Refreshing section")
		if err := c.refreshSection(ctx, serverURL, id, serverToken); err != nil {
			c.log.Warn().Err(err).Str("section", id).Msg("Refresh section failed")
			continue
		}
		refreshed++
		c.log.Debug().Str("section_id", id).Msg("Section refreshed")
	}

	c.log.Info().
		Int("sections_refreshed", refreshed).
		Int("sections_total", len(sectionIDs)).
		Msg("Plex library refresh complete")
	c.log.Debug().Msg("=== End of library refresh ===")

	return nil
}

// refreshSection refreshes a single library section
func (c *Client) refreshSection(ctx context.Context, serverURL, sectionID, token string) error {
	url := fmt.Sprintf("%s/library/sections/%s/refresh", serverURL, sectionID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("plex.refreshSection req: %w", err)
	}
	req.Header.Set("X-Plex-Token", token)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("plex.refreshSection do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("plex.refreshSection status: %d", resp.StatusCode)
	}

	return nil
}

// resolveServerURL fetches the Plex resources list and returns the best reachable
// server URL plus the per-server accessToken to use for subsequent PMS API calls.
// Connection priority per Plex docs: local -> non-relay -> relay (last resort).
func (c *Client) resolveServerURL(ctx context.Context, plexToken string) (string, string, error) {
	c.log.Debug().Msg("=== Resolving Plex server URL ===")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.resourcesURL, nil)
	if err != nil {
		return "", "", fmt.Errorf("plex.resolveServerURL req: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Plex-Token", plexToken)
	req.Header.Set("X-Plex-Client-Identifier", "filebot-webui")
	req.Header.Set("X-Plex-Product", "filebot-webui")

	c.log.Debug().Str("url", c.resourcesURL).Msg("Fetching Plex resources")

	resp, err := c.http.Do(req)
	if err != nil {
		c.log.Error().Err(err).Msg("Failed to fetch Plex resources")
		return "", "", fmt.Errorf("plex.resolveServerURL do: %w", err)
	}
	defer resp.Body.Close()

	c.log.Debug().Int("status_code", resp.StatusCode).Msg("Plex resources response")

	if resp.StatusCode != http.StatusOK {
		c.log.Error().Int("status", resp.StatusCode).Msg("Plex resources returned non-200 status")
		return "", "", fmt.Errorf("plex resources returned %d", resp.StatusCode)
	}

	type connection struct {
		URI   string `json:"uri"`
		Local bool   `json:"local"`
		Relay bool   `json:"relay"`
	}

	type resource struct {
		Provides    string       `json:"provides"`
		AccessToken string       `json:"accessToken"`
		Connections []connection `json:"connections"`
	}

	var resources []resource
	if err := json.NewDecoder(resp.Body).Decode(&resources); err != nil {
		c.log.Error().Err(err).Msg("Failed to decode Plex resources response")
		return "", "", fmt.Errorf("plex.resolveServerURL decode: %w", err)
	}

	c.log.Debug().Int("total_resources", len(resources)).Msg("Resources decoded")

	for idx, r := range resources {
		c.log.Debug().
			Int("resource_index", idx).
			Str("provides", r.Provides).
			Int("connections", len(r.Connections)).
			Msg("Processing resource")

		if r.Provides != "server" {
			c.log.Debug().Str("provides", r.Provides).Msg("Skipping non-server resource")
			continue
		}

		// Per docs: local preferred, relay last resort.
		var local, nonRelay, relay []connection
		for connIdx, conn := range r.Connections {
			c.log.Debug().
				Int("connection_index", connIdx).
				Str("uri", conn.URI).
				Bool("local", conn.Local).
				Bool("relay", conn.Relay).
				Msg("Processing connection")

			// Parse and log connection details
			c.logConnectionDetails(conn.URI)

			switch {
			case conn.Local:
				local = append(local, conn)
				c.log.Debug().Str("uri", conn.URI).Msg("Added to local connections")
			case conn.Relay:
				relay = append(relay, conn)
				c.log.Debug().Str("uri", conn.URI).Msg("Added to relay connections")
			default:
				nonRelay = append(nonRelay, conn)
				c.log.Debug().Str("uri", conn.URI).Msg("Added to non-relay connections")
			}
		}

		c.log.Debug().
			Int("local_count", len(local)).
			Int("non_relay_count", len(nonRelay)).
			Int("relay_count", len(relay)).
			Msg("Connection counts by type")

		// Use server's own accessToken; fall back to user token if absent.
		tok := r.AccessToken
		if tok == "" {
			tok = plexToken
			c.log.Debug().Msg("Using user token (no server-specific token)")
		} else {
			c.log.Debug().Str("token_prefix", c.maskToken(tok)).Msg("Using server-specific token")
		}

		// Try connections in order: local -> non-relay -> relay
		ordered := append(append(local, nonRelay...), relay...)
		c.log.Debug().Int("total_ordered", len(ordered)).Msg("Trying connections in priority order")

		for orderIdx, conn := range ordered {
			c.log.Debug().
				Int("priority_order", orderIdx+1).
				Int("total_connections", len(ordered)).
				Str("uri", conn.URI).
				Bool("local", conn.Local).
				Bool("relay", conn.Relay).
				Msg("Testing connection")

			// Check if the connection is reachable
			if c.isConnectionReachable(ctx, conn, tok) {
				c.log.Debug().
					Str("uri", conn.URI).
					Bool("local", conn.Local).
					Bool("relay", conn.Relay).
					Msg("Connection reachable, using this one")
				return conn.URI, tok, nil
			}
			c.log.Debug().
				Str("uri", conn.URI).
				Msg("Connection unreachable, trying next")
		}
	}

	c.log.Error().Msg("No reachable Plex server found")
	return "", "", fmt.Errorf("no reachable Plex server found")
}

// logConnectionDetails logs detailed information about a connection URI
func (c *Client) logConnectionDetails(rawURI string) {
	u, err := url.Parse(rawURI)
	if err != nil {
		c.log.Debug().Err(err).Str("uri", rawURI).Msg("Failed to parse URI")
		return
	}

	c.log.Debug().
		Str("scheme", u.Scheme).
		Str("host", u.Host).
		Str("hostname", u.Hostname()).
		Str("port", u.Port()).
		Str("path", u.Path).
		Msg("URI parsed")

	// Check if host is an IP or hostname
	host := u.Hostname()
	if ip := net.ParseIP(host); ip != nil {
		c.log.Debug().
			Str("ip", ip.String()).
			Bool("is_ipv4", ip.To4() != nil).
			Bool("is_ipv6", ip.To4() == nil && ip.To16() != nil).
			Msg("Host is an IP address")
	} else {
		c.log.Debug().
			Str("hostname", host).
			Bool("is_plex_direct", strings.Contains(host, "plex.direct")).
			Msg("Host is a hostname")

		// Check if it's a plex.direct hostname
		if strings.Contains(host, "plex.direct") {
			m := plexDirectRe.FindStringSubmatch(host)
			if m != nil {
				ip := strings.ReplaceAll(m[1], "-", ".")
				c.log.Debug().
					Str("extracted_ip_from_hostname", ip).
					Msg("Extracted IP from plex.direct hostname")
			}
		}
	}
}

// isConnectionReachable checks if a Plex server connection is reachable
func (c *Client) isConnectionReachable(ctx context.Context, conn struct {
	URI   string `json:"uri"`
	Local bool   `json:"local"`
	Relay bool   `json:"relay"`
}, plexToken string) bool {
	c.log.Debug().Str("uri", conn.URI).Msg("Checking connection reachability")

	// First, check if we can resolve DNS for this connection
	if !c.resolveConnectionDNS(ctx, conn.URI) {
		c.log.Debug().Str("uri", conn.URI).Msg("DNS resolution failed")
		return false
	}

	// Then probe the connection
	return c.probeConnection(ctx, conn.URI, plexToken)
}

// resolveConnectionDNS attempts to resolve the hostname in the URI
// Returns true if DNS resolution succeeds
func (c *Client) resolveConnectionDNS(ctx context.Context, rawURI string) bool {
	c.log.Debug().Str("uri", rawURI).Msg("Attempting DNS resolution")

	u, err := url.Parse(rawURI)
	if err != nil {
		c.log.Debug().Err(err).Str("uri", rawURI).Msg("Failed to parse URI for DNS resolution")
		return false
	}

	host := u.Hostname()
	if host == "" {
		c.log.Debug().Msg("No hostname found in URI")
		return false
	}

	c.log.Debug().Str("host", host).Msg("Resolving hostname")

	// Check if it's an IP address already
	if ip := net.ParseIP(host); ip != nil {
		c.log.Debug().
			Str("host", host).
			Str("ip", ip.String()).
			Msg("Host is already an IP address, no DNS resolution needed")
		return true
	}

	// For plex.direct domains, try to resolve
	ips, err := c.dnsResolver(ctx, host)
	if err != nil {
		c.log.Debug().
			Err(err).
			Str("host", host).
			Msg("DNS resolution failed")
		return false
	}

	if len(ips) == 0 {
		c.log.Debug().
			Str("host", host).
			Msg("DNS resolution returned no IPs")
		return false
	}

	c.log.Debug().
		Str("host", host).
		Strs("resolved_ips", ips).
		Int("ip_count", len(ips)).
		Msg("DNS resolution successful")

	// Log each resolved IP with its type
	for i, ipStr := range ips {
		if ip := net.ParseIP(ipStr); ip != nil {
			ipType := "unknown"
			if ip.To4() != nil {
				ipType = "IPv4"
			} else if ip.To16() != nil {
				ipType = "IPv6"
			}
			c.log.Debug().
				Int("index", i).
				Str("ip", ipStr).
				Str("type", ipType).
				Bool("is_private", isPrivateIP(ip)).
				Msg("Resolved IP details")
		}
	}

	return true
}

// isPrivateIP checks if an IP is private (RFC 1918, etc.)
func isPrivateIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}

	// Check private IPv4 ranges
	if ip.To4() != nil {
		// 10.0.0.0/8
		if ip[0] == 10 {
			return true
		}
		// 172.16.0.0/12
		if ip[0] == 172 && ip[1] >= 16 && ip[1] <= 31 {
			return true
		}
		// 192.168.0.0/16
		if ip[0] == 192 && ip[1] == 168 {
			return true
		}
		// 169.254.0.0/16
		if ip[0] == 169 && ip[1] == 254 {
			return true
		}
		// 127.0.0.0/8
		if ip[0] == 127 {
			return true
		}
	}

	return false
}

// probeConnection checks whether a Plex server URI is reachable by hitting /identity
func (c *Client) probeConnection(ctx context.Context, uri, plexToken string) bool {
	c.log.Debug().Str("uri", uri).Msg("Probing connection with HTTP HEAD")

	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(probeCtx, http.MethodHead, uri+"/identity", nil)
	if err != nil {
		c.log.Debug().Err(err).Msg("Failed to create probe request")
		return false
	}
	req.Header.Set("X-Plex-Token", plexToken)

	startTime := time.Now()
	resp, err := c.http.Do(req)
	duration := time.Since(startTime)

	if err != nil {
		c.log.Debug().
			Err(err).
			Dur("duration_ms", duration).
			Msg("Probe request failed")
		return false
	}
	defer resp.Body.Close()

	c.log.Debug().
		Int("status_code", resp.StatusCode).
		Dur("duration_ms", duration).
		Bool("success", resp.StatusCode < 500).
		Msg("Probe request completed")

	if resp.StatusCode < 500 {
		// Log response headers for debugging
		c.log.Debug().
			Str("content_type", resp.Header.Get("Content-Type")).
			Str("server", resp.Header.Get("Server")).
			Str("x_plex_version", resp.Header.Get("X-Plex-Version")).
			Msg("Probe response headers")
	}

	return resp.StatusCode < 500
}

// maskToken masks a token for logging (shows first 4 and last 4 chars)
func (c *Client) maskToken(token string) string {
	if len(token) <= 8 {
		return "****"
	}
	return token[:4] + "****" + token[len(token)-4:]
}

// listSections returns all library section IDs
func (c *Client) listSections(ctx context.Context, serverURL, plexToken string) ([]string, error) {
	c.log.Debug().Str("server_url", serverURL).Msg("Fetching library sections")

	url := serverURL + "/library/sections"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("plex.listSections req: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Plex-Token", plexToken)

	resp, err := c.http.Do(req)
	if err != nil {
		c.log.Error().Err(err).Msg("Failed to fetch library sections")
		return nil, fmt.Errorf("plex.listSections do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.log.Error().Int("status", resp.StatusCode).Msg("Library sections returned non-200 status")
		return nil, fmt.Errorf("plex.listSections status: %d", resp.StatusCode)
	}

	var body struct {
		MediaContainer struct {
			Directory []struct {
				Key      string `json:"key"`
				Title    string `json:"title"`
				Type     string `json:"type"`
				Location []struct {
					Path string `json:"path"`
				} `json:"Location"`
			} `json:"Directory"`
		} `json:"MediaContainer"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		c.log.Error().Err(err).Msg("Failed to decode library sections response")
		return nil, fmt.Errorf("plex.listSections decode: %w", err)
	}

	var ids []string
	for idx, d := range body.MediaContainer.Directory {
		if d.Key != "" {
			ids = append(ids, d.Key)
			c.log.Debug().
				Int("index", idx).
				Str("section_id", d.Key).
				Str("title", d.Title).
				Str("type", d.Type).
				Msg("Found library section")
			if len(d.Location) > 0 {
				c.log.Debug().
					Str("section_id", d.Key).
					Str("location", d.Location[0].Path).
					Msg("Section location")
			}
		}
	}

	c.log.Debug().Int("total_sections", len(ids)).Msg("Library sections fetched")
	return ids, nil
}
