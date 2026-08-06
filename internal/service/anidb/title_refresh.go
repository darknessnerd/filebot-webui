package anidb

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/darknessnerd/filebot-webui/internal/logger"
)

// RefreshTitlesFile downloads the AniDB titles XML from sourceURL, validates
// it, writes it atomically to targetPath, and returns the raw XML bytes so
// callers can reload an in-memory index without a second disk read.
func RefreshTitlesFile(ctx context.Context, sourceURL, targetPath string, log logger.Logger) ([]byte, error) {
	sourceURL = strings.TrimSpace(sourceURL)
	targetPath = strings.TrimSpace(targetPath)
	if sourceURL == "" {
		return nil, fmt.Errorf("anidb.RefreshTitlesFile: source URL is required")
	}
	if targetPath == "" {
		return nil, fmt.Errorf("anidb.RefreshTitlesFile: target path is required")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return nil, fmt.Errorf("anidb.RefreshTitlesFile request: %w", err)
	}
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; filebot-webui/1.0; +https://github.com/darknessnerd/filebot-webui)")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("anidb.RefreshTitlesFile do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("anidb.RefreshTitlesFile status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, fmt.Errorf("anidb.RefreshTitlesFile read: %w", err)
	}

	content, err := maybeGunzip(raw, resp.Header.Get("Content-Encoding"), sourceURL)
	if err != nil {
		return nil, fmt.Errorf("anidb.RefreshTitlesFile decompress: %w", err)
	}
	if !bytes.Contains(content, []byte("<animetitles")) {
		return nil, fmt.Errorf("anidb.RefreshTitlesFile: payload does not look like anime titles XML")
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return nil, fmt.Errorf("anidb.RefreshTitlesFile mkdir: %w", err)
	}
	tmp := targetPath + ".tmp"
	if err := os.WriteFile(tmp, content, 0o644); err != nil {
		return nil, fmt.Errorf("anidb.RefreshTitlesFile write tmp: %w", err)
	}
	defer os.Remove(tmp) // no-op if Rename succeeded; cleans up on failure
	if err := os.Rename(tmp, targetPath); err != nil {
		return nil, fmt.Errorf("anidb.RefreshTitlesFile replace: %w", err)
	}

	log.Info().Str("path", targetPath).Int("bytes", len(content)).Msg("anidb: titles file refreshed")
	return content, nil
}

func maybeGunzip(raw []byte, contentEncoding, sourceURL string) ([]byte, error) {
	isGzip := strings.Contains(strings.ToLower(contentEncoding), "gzip") || strings.HasSuffix(strings.ToLower(sourceURL), ".gz")
	if !isGzip {
		return raw, nil
	}
	r, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}
