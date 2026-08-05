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

func RefreshTitlesFile(ctx context.Context, sourceURL, targetPath string, log logger.Logger) error {
	sourceURL = strings.TrimSpace(sourceURL)
	targetPath = strings.TrimSpace(targetPath)
	if sourceURL == "" {
		return fmt.Errorf("anidb.RefreshTitlesFile: source URL is required")
	}
	if targetPath == "" {
		return fmt.Errorf("anidb.RefreshTitlesFile: target path is required")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return fmt.Errorf("anidb.RefreshTitlesFile request: %w", err)
	}
	req.Header.Set("Accept-Encoding", "gzip")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("anidb.RefreshTitlesFile do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("anidb.RefreshTitlesFile status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return fmt.Errorf("anidb.RefreshTitlesFile read: %w", err)
	}

	content, err := maybeGunzip(raw, resp.Header.Get("Content-Encoding"), sourceURL)
	if err != nil {
		return fmt.Errorf("anidb.RefreshTitlesFile decompress: %w", err)
	}
	if !bytes.Contains(content, []byte("<animetitles")) {
		return fmt.Errorf("anidb.RefreshTitlesFile: payload does not look like anime titles XML")
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("anidb.RefreshTitlesFile mkdir: %w", err)
	}
	tmp := targetPath + ".tmp"
	if err := os.WriteFile(tmp, content, 0o644); err != nil {
		return fmt.Errorf("anidb.RefreshTitlesFile write tmp: %w", err)
	}
	if err := os.Rename(tmp, targetPath); err != nil {
		return fmt.Errorf("anidb.RefreshTitlesFile replace: %w", err)
	}

	log.Info().Str("path", targetPath).Int("bytes", len(content)).Msg("anidb: titles file refreshed")
	return nil
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
