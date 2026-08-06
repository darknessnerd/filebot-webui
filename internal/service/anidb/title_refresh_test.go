package anidb

import (
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/darknessnerd/filebot-webui/internal/logger"
)

func TestRefreshTitlesFile_PlainXML(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<?xml version="1.0"?><animetitles><anime aid="1"><title>One Piece</title></anime></animetitles>`))
	}))
	defer srv.Close()

	target := filepath.Join(t.TempDir(), "anime-titles.xml")
	content, err := RefreshTitlesFile(context.Background(), srv.URL, target, logger.New("error", false))
	require.NoError(t, err)
	assert.Contains(t, string(content), "<animetitles>")

	data, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Contains(t, string(data), "<animetitles>")
}

func TestRefreshTitlesFile_GzipXML(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		var buf bytes.Buffer
		zw := gzip.NewWriter(&buf)
		_, _ = zw.Write([]byte(`<?xml version="1.0"?><animetitles><anime aid="2"><title>Dirty Pair Flash</title></anime></animetitles>`))
		_ = zw.Close()
		w.Header().Set("Content-Encoding", "gzip")
		_, _ = w.Write(buf.Bytes())
	}))
	defer srv.Close()

	target := filepath.Join(t.TempDir(), "anime-titles.xml")
	content, err := RefreshTitlesFile(context.Background(), srv.URL, target, logger.New("error", false))
	require.NoError(t, err)
	assert.Contains(t, string(content), "Dirty Pair Flash")

	data, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Contains(t, string(data), "Dirty Pair Flash")
}
