package plex

import (
	"context"
	"encoding/json"
	"fmt"
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

type Client struct {
	http         *http.Client
	log          logger.Logger
	resourcesURL string
}

func NewClient(log logger.Logger) *Client {
	return &Client{
		http:         &http.Client{Timeout: 15 * time.Second},
		log:          log,
		resourcesURL: "https://clients.plex.tv/api/v2/resources?includeHttps=1&includeRelay=1&includeIPv6=1",
	}
}

func (c *Client) RefreshLibraries(ctx context.Context, plexToken string) error {
	c.log.Debug().Msg("plex: resolving server URL")
	serverURL, serverToken, err := c.resolveServerURL(ctx, plexToken)
	if err != nil {
		return fmt.Errorf("plex.RefreshLibraries resolve: %w", err)
	}
	c.log.Debug().Str("server_url", serverURL).Msg("plex: server resolved")

	sectionIDs, err := c.listSections(ctx, serverURL, serverToken)
	if err != nil {
		return fmt.Errorf("plex.RefreshLibraries sections: %w", err)
	}
	c.log.Debug().Int("sections", len(sectionIDs)).Msg("plex: sections listed")

	refreshed := 0
	for _, id := range sectionIDs {
		url := fmt.Sprintf("%s/library/sections/%s/refresh", serverURL, id)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return fmt.Errorf("plex.RefreshLibraries refresh req: %w", err)
		}
		req.Header.Set("X-Plex-Token", serverToken)
		resp, err := c.http.Do(req)
		if err != nil {
			c.log.Warn().Err(err).Str("section", id).Msg("plex refresh section failed")
			continue
		}
		resp.Body.Close()
		refreshed++
	}
	c.log.Info().Int("sections_refreshed", refreshed).Int("sections_total", len(sectionIDs)).Msg("plex: library refresh complete")
	return nil
}

// resolveServerURL fetches the Plex resources list and returns the best reachable
// server URL plus the per-server accessToken to use for subsequent PMS API calls.
// Connection priority per Plex docs: local → non-relay → relay (last resort).
func (c *Client) resolveServerURL(ctx context.Context, plexToken string) (string, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.resourcesURL, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Plex-Token", plexToken)
	req.Header.Set("X-Plex-Client-Identifier", "filebot-webui")
	req.Header.Set("X-Plex-Product", "filebot-webui")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("plex resources returned %d", resp.StatusCode)
	}

	var resources []struct {
		Provides    string `json:"provides"`
		AccessToken string `json:"accessToken"`
		Connections []struct {
			URI   string `json:"uri"`
			Local bool   `json:"local"`
			Relay bool   `json:"relay"`
		} `json:"connections"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&resources); err != nil {
		return "", "", err
	}

	for _, r := range resources {
		if r.Provides != "server" {
			continue
		}
		// Per docs: local preferred, relay last resort.
		var local, nonRelay, relay []string
		for _, conn := range r.Connections {
			c.log.Debug().Str("uri", conn.URI).Bool("local", conn.Local).Bool("relay", conn.Relay).Msg("plex: candidate connection")
			switch {
			case conn.Local:
				local = append(local, conn.URI)
			case conn.Relay:
				relay = append(relay, conn.URI)
			default:
				nonRelay = append(nonRelay, conn.URI)
			}
		}
		ordered := append(append(local, nonRelay...), relay...)

		// Use server's own accessToken; fall back to user token if absent.
		tok := r.AccessToken
		if tok == "" {
			tok = plexToken
		}

		// For plex.direct URIs that may not resolve via DNS, also try direct IP.
		var expanded []string
		for _, uri := range ordered {
			expanded = append(expanded, uri)
			if alt := plexDirectToIP(uri); alt != "" {
				expanded = append(expanded, alt)
			}
		}

		for _, uri := range expanded {
			if c.probeConnection(ctx, uri, tok) {
				c.log.Debug().Str("uri", uri).Msg("plex: connection reachable")
				return uri, tok, nil
			}
			c.log.Debug().Str("uri", uri).Msg("plex: connection unreachable, trying next")
		}
	}
	return "", "", fmt.Errorf("no reachable Plex server found")
}

// plexDirectToIP converts a plex.direct URI to a direct-IP URI, or returns "".
// e.g. https://192-168-178-51.<hash>.plex.direct:32400 → https://192.168.178.51:32400
func plexDirectToIP(rawURI string) string {
	u, err := url.Parse(rawURI)
	if err != nil {
		return ""
	}
	host := u.Hostname()
	m := plexDirectRe.FindStringSubmatch(host)
	if m == nil {
		return ""
	}
	ip := strings.ReplaceAll(m[1], "-", ".")
	port := u.Port()
	if port != "" {
		u.Host = ip + ":" + port
	} else {
		u.Host = ip
	}
	return u.String()
}

// probeConnection checks whether a Plex server URI is reachable by hitting /identity.
func (c *Client) probeConnection(ctx context.Context, uri, plexToken string) bool {
	probeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, uri+"/identity", nil)
	if err != nil {
		return false
	}
	req.Header.Set("X-Plex-Token", plexToken)

	resp, err := c.http.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode < 500
}

func (c *Client) listSections(ctx context.Context, serverURL, plexToken string) ([]string, error) {
	url := serverURL + "/library/sections"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Plex-Token", plexToken)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var body struct {
		MediaContainer struct {
			Directory []struct {
				Key string `json:"key"`
			} `json:"Directory"`
		} `json:"MediaContainer"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}

	var ids []string
	for _, d := range body.MediaContainer.Directory {
		ids = append(ids, d.Key)
	}
	return ids, nil
}
