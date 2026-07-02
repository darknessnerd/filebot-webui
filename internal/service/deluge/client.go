package deluge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/darknessnerd/filebot-webui/internal/domain"
	"github.com/darknessnerd/filebot-webui/internal/logger"
)

type Client struct {
	baseURL  string
	password string
	http     *http.Client
	log      logger.Logger
}

func NewClient(host, port, password string, log logger.Logger) *Client {
	return &Client{
		baseURL:  fmt.Sprintf("http://%s:%s/json", host, port),
		password: password,
		http:     &http.Client{Timeout: 10 * time.Second},
		log:      log,
	}
}

func (c *Client) ListCompleted(ctx context.Context) ([]domain.Torrent, error) {
	c.log.Debug().Msg("deluge: authenticating")
	cookie, err := c.authenticate(ctx)
	if err != nil {
		return nil, fmt.Errorf("deluge.ListCompleted: %w", err)
	}

	fields := []string{
		"name", "state", "progress", "total_size",
		"download_location", "is_finished", "completed_time",
	}
	c.log.Debug().Msg("deluge: fetching torrent list")
	resp, err := c.rpc(ctx, cookie, "web.update_ui", []any{fields, map[string]any{}})
	if err != nil {
		return nil, fmt.Errorf("deluge.ListCompleted: %w", err)
	}

	result, _ := resp["result"].(map[string]any)
	torrentsRaw, _ := result["torrents"].(map[string]any)

	var out []domain.Torrent
	for id, raw := range torrentsRaw {
		d, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		isFinished, _ := d["is_finished"].(bool)
		progress, _ := d["progress"].(float64)
		if !isFinished || progress < 100 {
			continue
		}

		completedUnix, _ := d["completed_time"].(float64)
		out = append(out, domain.Torrent{
			ID:           id,
			Name:         strField(d, "name"),
			State:        strField(d, "state"),
			Progress:     progress,
			DownloadPath: strField(d, "download_location"),
			Size:         int64Field(d, "total_size"),
			IsFinished:   true,
			CompletedOn:  time.Unix(int64(completedUnix), 0),
		})
	}
	c.log.Info().Int("count", len(out)).Msg("deluge: completed torrents fetched")
	return out, nil
}

func (c *Client) DeleteTorrent(ctx context.Context, id string) error {
	c.log.Debug().Str("torrent_id", id).Msg("deluge: deleting torrent")
	cookie, err := c.authenticate(ctx)
	if err != nil {
		return fmt.Errorf("deluge.DeleteTorrent: %w", err)
	}
	resp, err := c.rpc(ctx, cookie, "core.remove_torrent", []any{id, true})
	if err != nil {
		return fmt.Errorf("deluge.DeleteTorrent: %w", err)
	}
	if resp["error"] != nil {
		return fmt.Errorf("deluge.DeleteTorrent: %v", resp["error"])
	}
	c.log.Info().Str("torrent_id", id).Msg("deluge: torrent deleted")
	return nil
}

func (c *Client) authenticate(ctx context.Context) (*http.Cookie, error) {
	body, err := jsonBody(map[string]any{
		"method": "auth.login",
		"params": []string{c.password},
		"id":     1,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrDelugeUnavailable, err)
	}
	defer resp.Body.Close()

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	ok, _ := result["result"].(bool)
	if !ok {
		return nil, fmt.Errorf("%w: auth.login returned false", domain.ErrDelugeUnavailable)
	}

	for _, ck := range resp.Cookies() {
		if ck.Name == "_session_id" {
			return ck, nil
		}
	}
	return nil, fmt.Errorf("%w: no session cookie", domain.ErrDelugeUnavailable)
}

func (c *Client) rpc(ctx context.Context, cookie *http.Cookie, method string, params []any) (map[string]any, error) {
	body, err := jsonBody(map[string]any{
		"method": method,
		"params": params,
		"id":     2,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func jsonBody(v any) (*bytes.Buffer, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return bytes.NewBuffer(b), nil
}

func strField(m map[string]any, k string) string {
	v, _ := m[k].(string)
	return v
}

func int64Field(m map[string]any, k string) int64 {
	v, _ := m[k].(float64)
	return int64(v)
}
