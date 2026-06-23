package plex

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/darknessnerd/filebot-webui/internal/logger"
)

type Client struct {
	http *http.Client
	log  logger.Logger
}

func NewClient(log logger.Logger) *Client {
	return &Client{
		http: &http.Client{Timeout: 15 * time.Second},
		log:  log,
	}
}

func (c *Client) RefreshLibraries(ctx context.Context, plexToken string) error {
	serverURL, err := c.resolveServerURL(ctx, plexToken)
	if err != nil {
		return fmt.Errorf("plex.RefreshLibraries resolve: %w", err)
	}

	sectionIDs, err := c.listSections(ctx, serverURL, plexToken)
	if err != nil {
		return fmt.Errorf("plex.RefreshLibraries sections: %w", err)
	}

	for _, id := range sectionIDs {
		url := fmt.Sprintf("%s/library/sections/%s/refresh", serverURL, id)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return fmt.Errorf("plex.RefreshLibraries refresh req: %w", err)
		}
		req.Header.Set("X-Plex-Token", plexToken)
		resp, err := c.http.Do(req)
		if err != nil {
			c.log.Warn().Err(err).Str("section", id).Msg("plex refresh section failed")
			continue
		}
		resp.Body.Close()
	}
	return nil
}

func (c *Client) resolveServerURL(ctx context.Context, plexToken string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://plex.tv/api/v2/resources?includeHttps=1", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Plex-Token", plexToken)
	req.Header.Set("X-Plex-Client-Identifier", "filebot-webui")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("plex resources returned %d", resp.StatusCode)
	}

	var resources []struct {
		Provides    string `json:"provides"`
		Connections []struct {
			URI   string `json:"uri"`
			Local bool   `json:"local"`
		} `json:"connections"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&resources); err != nil {
		return "", err
	}

	for _, r := range resources {
		if r.Provides != "server" {
			continue
		}
		for _, conn := range r.Connections {
			if !conn.Local {
				return conn.URI, nil
			}
		}
		if len(r.Connections) > 0 {
			return r.Connections[0].URI, nil
		}
	}
	return "", fmt.Errorf("no Plex server found in resources")
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
