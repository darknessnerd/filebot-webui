package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── createPlexPin ─────────────────────────────────────────────────────────────

func TestCreatePlexPin_Happy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"id": 42, "code": "ABCD1234"})
	}))
	defer srv.Close()

	// patch the URL used by createPlexPin via a round-trip through a local transport
	id, code, err := createPlexPinURL(srv.URL+"/pins", "clientid", "appname")
	require.NoError(t, err)
	assert.Equal(t, int64(42), id)
	assert.Equal(t, "ABCD1234", code)
}

func TestCreatePlexPin_BadJSON_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer srv.Close()

	_, _, err := createPlexPinURL(srv.URL+"/pins", "clientid", "appname")
	assert.Error(t, err)
}

// ── pollPlexPin ───────────────────────────────────────────────────────────────

func TestPollPlexPin_ImmediateToken(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"authToken": "tok-xyz"})
	}))
	defer srv.Close()

	tok, err := pollPlexPinURL(context.Background(), srv.URL+"/pins/1", "code", "clientid", 5*time.Second)
	require.NoError(t, err)
	assert.Equal(t, "tok-xyz", tok)
	assert.Equal(t, 1, calls)
}

func TestPollPlexPin_ContextCancelled_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"authToken": ""})
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := pollPlexPinURL(ctx, srv.URL+"/pins/1", "code", "clientid", 10*time.Second)
	assert.Error(t, err)
}

func TestPollPlexPin_Timeout_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"authToken": ""})
	}))
	defer srv.Close()

	_, err := pollPlexPinURL(context.Background(), srv.URL+"/pins/1", "code", "clientid", 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expired")
}

func TestPollPlexPin_BodyReadError_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// close connection mid-response to force a read error
		hj, ok := w.(http.Hijacker)
		if !ok {
			w.WriteHeader(http.StatusOK)
			return
		}
		conn, _, _ := hj.Hijack()
		conn.Close()
	}))
	defer srv.Close()

	_, err := pollPlexPinURL(context.Background(), srv.URL+"/pins/1", "code", "clientid", 5*time.Second)
	assert.Error(t, err)
}

// ── getPlexUserInfo ───────────────────────────────────────────────────────────

func TestGetPlexUserInfo_Happy_NumericID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id":       float64(99),
			"username": "alice",
			"email":    "alice@example.com",
			"thumb":    "https://plex.tv/avatar.png",
		})
	}))
	defer srv.Close()

	info, err := getPlexUserInfoURL(srv.URL+"/user", "clientid", "appname", "token")
	require.NoError(t, err)
	assert.Equal(t, "99", info.id)
	assert.Equal(t, "alice", info.username)
	assert.Equal(t, "alice@example.com", info.email)
}

func TestGetPlexUserInfo_Happy_StringID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"id": "str-id", "username": "bob"})
	}))
	defer srv.Close()

	info, err := getPlexUserInfoURL(srv.URL+"/user", "clientid", "appname", "token")
	require.NoError(t, err)
	assert.Equal(t, "str-id", info.id)
}

func TestGetPlexUserInfo_NonOKStatus_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	_, err := getPlexUserInfoURL(srv.URL+"/user", "clientid", "appname", "token")
	assert.Error(t, err)
}

func TestGetPlexUserInfo_BadJSON_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{broken"))
	}))
	defer srv.Close()

	_, err := getPlexUserInfoURL(srv.URL+"/user", "clientid", "appname", "token")
	assert.Error(t, err)
}
