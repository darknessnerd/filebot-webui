package anidb

import (
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

func TestSearchAnime_ParsesAnimeResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "anime", r.URL.Query().Get("request"))
		assert.Equal(t, "myclient", r.URL.Query().Get("client"))
		assert.Equal(t, "1", r.URL.Query().Get("clientver"))
		assert.Equal(t, "1", r.URL.Query().Get("protover"))
		assert.Equal(t, "12345", r.URL.Query().Get("aid"))
		_, _ = w.Write([]byte(`
<anime id="12345">
  <titles>
    <title type="synonym">Mushoku</title>
    <title type="main">Mushoku Tensei: Jobless Reincarnation</title>
  </titles>
  <startdate>2021-01-10</startdate>
</anime>`))
	}))
	defer srv.Close()

	c := NewClient("myclient", "1", "1", srv.URL, "", logger.New("error", false))
	match, err := c.SearchAnimeByAID(context.Background(), 12345)
	require.NoError(t, err)
	require.NotNil(t, match)
	assert.Equal(t, 12345, match.ID)
	assert.Equal(t, "Mushoku Tensei: Jobless Reincarnation", match.Title)
	assert.Equal(t, 2021, match.Year)
}

func TestSearchAnime_ApiErrorPayload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<error code="330">Banned</error>`))
	}))
	defer srv.Close()

	c := NewClient("myclient", "1", "1", srv.URL, "", logger.New("error", false))
	_, err := c.SearchAnimeByAID(context.Background(), 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "code 330")
}

func TestSearchAnime_RequiresClientConfig(t *testing.T) {
	c := NewClient("", "", "1", "http://example.invalid", "", logger.New("error", false))
	_, err := c.SearchAnimeByAID(context.Background(), 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "client and clientver are required")
}

func TestSearchAnimeByAID_UsesCache(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		_, _ = w.Write([]byte(`
<anime id="42">
  <titles>
    <title type="main">Answer</title>
  </titles>
  <startdate>1979-01-01</startdate>
</anime>`))
	}))
	defer srv.Close()

	c := NewClient("myclient", "1", "1", srv.URL, "", logger.New("error", false))
	first, err := c.SearchAnimeByAID(context.Background(), 42)
	require.NoError(t, err)
	second, err := c.SearchAnimeByAID(context.Background(), 42)
	require.NoError(t, err)

	assert.Equal(t, 1, calls)
	assert.Equal(t, first, second)
}

func TestSearchAnime_ResolvesTitleViaIndex(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		assert.Equal(t, "42", r.URL.Query().Get("aid"))
		_, _ = w.Write([]byte(`
<anime id="42">
  <titles>
    <title type="main">Dirty Pair Flash</title>
  </titles>
  <startdate>1994-01-01</startdate>
</anime>`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	titlesPath := filepath.Join(dir, "anime-titles.xml")
	require.NoError(t, os.WriteFile(titlesPath, []byte(`<?xml version="1.0" encoding="UTF-8"?>
<animetitles>
  <anime aid="42"><title type="main">Dirty Pair Flash</title></anime>
</animetitles>`), 0644))

	c := NewClient("myclient", "1", "1", srv.URL, titlesPath, logger.New("error", false))
	match, err := c.SearchAnime(context.Background(), "Dirty Pair Flash", 1994)
	require.NoError(t, err)
	assert.Equal(t, 42, match.ID)
	assert.Equal(t, 1, calls)
}
