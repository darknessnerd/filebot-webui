package tmdb

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/darknessnerd/filebot-webui/internal/logger"
)

func TestSearchMovie(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/search/movie", r.URL.Path)
		assert.Equal(t, "Dune Part Two", r.URL.Query().Get("query"))
		assert.Equal(t, "2024", r.URL.Query().Get("primary_release_year"))
		assert.Equal(t, "Bearer tok", r.Header.Get("Authorization"))
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{"id": 12, "title": "Dune: Part Two", "release_date": "2024-03-01", "popularity": 100},
			},
		}))
	}))
	defer srv.Close()

	client := NewClient("tok", logger.New("error", false))
	client.baseURL = srv.URL

	match, err := client.SearchMovie(context.Background(), "Dune Part Two", 2024)
	require.NoError(t, err)
	assert.Equal(t, "Dune: Part Two", match.Title)
	assert.Equal(t, 2024, match.Year)
}

func TestSearchTV(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/search/tv", r.URL.Path)
		assert.Equal(t, "Breaking Bad", r.URL.Query().Get("query"))
		assert.Equal(t, "2008", r.URL.Query().Get("first_air_date_year"))
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{"id": 1396, "name": "Breaking Bad", "first_air_date": "2008-01-20", "popularity": 100},
			},
		}))
	}))
	defer srv.Close()

	client := NewClient("tok", logger.New("error", false))
	client.baseURL = srv.URL

	match, err := client.SearchTV(context.Background(), "Breaking Bad", 2008)
	require.NoError(t, err)
	assert.Equal(t, "Breaking Bad", match.Name)
	assert.Equal(t, 2008, match.Year)
}
