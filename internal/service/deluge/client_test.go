package deluge

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/darknessnerd/filebot-webui/internal/domain"
	"github.com/darknessnerd/filebot-webui/internal/logger"
)

func newMockDeluge(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	c := &Client{
		baseURL:  srv.URL,
		password: "secret",
		http:     srv.Client(),
		log:      logger.New("error", false),
	}
	return c, srv
}

func authOKHandler(next http.HandlerFunc) http.HandlerFunc {
	called := false
	return func(w http.ResponseWriter, r *http.Request) {
		if !called {
			called = true
			http.SetCookie(w, &http.Cookie{Name: "_session_id", Value: "abc"})
			json.NewEncoder(w).Encode(map[string]any{"result": true, "error": nil, "id": 1})
			return
		}
		next(w, r)
	}
}

func TestListCompleted_FiltersIncomplete(t *testing.T) {
	handler := authOKHandler(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"result": map[string]any{
				"torrents": map[string]any{
					"abc": map[string]any{
						"name": "Done Movie", "state": "Seeding", "progress": float64(100),
						"total_size": float64(1024), "download_location": "/downloads",
						"is_finished": true, "completed_time": float64(0),
					},
					"def": map[string]any{
						"name": "Partial", "state": "Downloading", "progress": float64(50),
						"total_size": float64(2048), "download_location": "/downloads",
						"is_finished": false, "completed_time": float64(0),
					},
				},
			},
			"error": nil, "id": 2,
		})
	})

	c, _ := newMockDeluge(t, handler)
	torrents, err := c.ListCompleted(context.Background())
	require.NoError(t, err)
	require.Len(t, torrents, 1)
	assert.Equal(t, "abc", torrents[0].ID)
	assert.Equal(t, "Done Movie", torrents[0].Name)
}

func TestDeleteTorrent_SendsCorrectPayload(t *testing.T) {
	var capturedBody map[string]any

	handler := authOKHandler(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedBody)
		json.NewEncoder(w).Encode(map[string]any{"result": true, "error": nil, "id": 2})
	})

	c, _ := newMockDeluge(t, handler)
	err := c.DeleteTorrent(context.Background(), "torrent-xyz")
	require.NoError(t, err)

	assert.Equal(t, "core.remove_torrent", capturedBody["method"])
	params, _ := capturedBody["params"].([]any)
	require.Len(t, params, 2)
	assert.Equal(t, "torrent-xyz", params[0])
	assert.Equal(t, true, params[1])
}

func TestListCompleted_AuthFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"result": false, "error": "bad pass", "id": 1})
	}))
	defer srv.Close()

	c := &Client{baseURL: srv.URL, password: "wrong", http: srv.Client(), log: logger.New("error", false)}
	_, err := c.ListCompleted(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrDelugeUnavailable)
}
