package plex

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/darknessnerd/filebot-webui/internal/logger"
)

// newTestClient returns a Client pointing the resources endpoint at resourcesURL,
// using the provided HTTP client (typically srv.Client() for test server routing).
func newTestClient(httpClient *http.Client, resourcesURL string) *Client {
	return &Client{
		http:         httpClient,
		log:          logger.New("error", false),
		resourcesURL: resourcesURL,
		dnsResolver:  net.DefaultResolver.LookupHost,
	}
}

func TestRefreshLibraries_CallsRefreshPerSection(t *testing.T) {
	refreshed := []string{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v2/resources":
			json.NewEncoder(w).Encode([]map[string]any{{
				"provides":    "server",
				"accessToken": "server-tok",
				"connections": []map[string]any{
					{"uri": "http://" + r.Host, "local": true},
				},
			}})
		case r.URL.Path == "/identity":
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/library/sections":
			json.NewEncoder(w).Encode(map[string]any{
				"MediaContainer": map[string]any{
					"Directory": []map[string]any{
						{"key": "1"},
						{"key": "2"},
					},
				},
			})
		case strings.HasPrefix(r.URL.Path, "/library/sections/") && strings.HasSuffix(r.URL.Path, "/refresh"):
			parts := strings.Split(r.URL.Path, "/")
			refreshed = append(refreshed, parts[3])
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	c := newTestClient(srv.Client(), srv.URL+"/api/v2/resources")
	err := c.RefreshLibraries(context.Background(), "user-tok")
	require.NoError(t, err)
	assert.Equal(t, []string{"1", "2"}, refreshed)
}

func TestRefreshLibraries_UsesServerAccessToken(t *testing.T) {
	var gotToken string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v2/resources":
			json.NewEncoder(w).Encode([]map[string]any{{
				"provides":    "server",
				"accessToken": "per-server-tok",
				"connections": []map[string]any{
					{"uri": "http://" + r.Host, "local": true},
				},
			}})
		case r.URL.Path == "/identity":
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/library/sections":
			json.NewEncoder(w).Encode(map[string]any{
				"MediaContainer": map[string]any{
					"Directory": []map[string]any{{"key": "3"}},
				},
			})
		case strings.HasPrefix(r.URL.Path, "/library/sections/") && strings.HasSuffix(r.URL.Path, "/refresh"):
			gotToken = r.Header.Get("X-Plex-Token")
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	c := newTestClient(srv.Client(), srv.URL+"/api/v2/resources")
	err := c.RefreshLibraries(context.Background(), "user-tok")
	require.NoError(t, err)
	assert.Equal(t, "per-server-tok", gotToken)
}

func TestRefreshLibraries_FallsBackToUserTokenWhenNoAccessToken(t *testing.T) {
	var gotToken string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v2/resources":
			// no accessToken → fall back to user token
			json.NewEncoder(w).Encode([]map[string]any{{
				"provides": "server",
				"connections": []map[string]any{
					{"uri": "http://" + r.Host, "local": true},
				},
			}})
		case r.URL.Path == "/identity":
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/library/sections":
			json.NewEncoder(w).Encode(map[string]any{
				"MediaContainer": map[string]any{
					"Directory": []map[string]any{{"key": "5"}},
				},
			})
		case strings.HasPrefix(r.URL.Path, "/library/sections/") && strings.HasSuffix(r.URL.Path, "/refresh"):
			gotToken = r.Header.Get("X-Plex-Token")
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	c := newTestClient(srv.Client(), srv.URL+"/api/v2/resources")
	err := c.RefreshLibraries(context.Background(), "user-tok")
	require.NoError(t, err)
	assert.Equal(t, "user-tok", gotToken)
}

func TestRefreshLibraries_NoReachableServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/resources" {
			json.NewEncoder(w).Encode([]map[string]any{{
				"provides": "server",
				"connections": []map[string]any{
					{"uri": "http://127.0.0.1:1", "local": true},
				},
			}})
		}
	}))
	defer srv.Close()

	c := newTestClient(srv.Client(), srv.URL+"/api/v2/resources")
	err := c.RefreshLibraries(context.Background(), "tok")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no reachable Plex server")
}

func TestRefreshLibraries_NetworkError(t *testing.T) {
	c := &Client{
		http:         &http.Client{},
		log:          logger.New("error", false),
		resourcesURL: "http://127.0.0.1:1/api/v2/resources",
	}
	err := c.RefreshLibraries(context.Background(), "token")
	assert.Error(t, err)
}

func TestResolveServerURL_ConnectionOrdering(t *testing.T) {
	// Connections returned in reverse priority order; resolveServerURL must probe local first.
	var probed []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/resources":
			json.NewEncoder(w).Encode([]map[string]any{{
				"provides": "server",
				"connections": []map[string]any{
					{"uri": fmt.Sprintf("http://%s/relay", r.Host), "local": false, "relay": true},
					{"uri": fmt.Sprintf("http://%s/nonrelay", r.Host), "local": false, "relay": false},
					{"uri": fmt.Sprintf("http://%s/local", r.Host), "local": true, "relay": false},
				},
			}})
		case "/local/identity", "/nonrelay/identity", "/relay/identity":
			probed = append(probed, strings.TrimSuffix(r.URL.Path, "/identity"))
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	c := newTestClient(srv.Client(), srv.URL+"/api/v2/resources")
	resolvedURL, _, err := c.resolveServerURL(context.Background(), "tok")
	require.NoError(t, err)
	require.NotEmpty(t, probed)
	assert.Equal(t, "/local", probed[0], "local connection must be tried first")
	assert.Contains(t, resolvedURL, "/local")
}

func TestResolveServerURL_FailsWhenPlexDirectDNSUnresolvable(t *testing.T) {
	fakePlexDirect := "http://127-0-0-1.aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa0.plex.direct:32400"

	resourcesSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]any{{
			"provides": "server",
			"connections": []map[string]any{
				{"uri": fakePlexDirect, "local": true},
			},
		}})
	}))
	defer resourcesSrv.Close()

	c := newTestClient(resourcesSrv.Client(), resourcesSrv.URL+"/api/v2/resources")
	c.dnsResolver = func(_ context.Context, host string) ([]string, error) {
		return nil, fmt.Errorf("dns failed for %s", host)
	}

	resolvedURL, _, err := c.resolveServerURL(context.Background(), "tok")
	require.Error(t, err)
	assert.Empty(t, resolvedURL)
	assert.Contains(t, err.Error(), "no reachable Plex server")
}

func TestResolveServerURL_UsesPlexDirectEmbeddedIPWhenDNSFails(t *testing.T) {
	plexSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/identity":
			w.WriteHeader(http.StatusOK)
		case "/library/sections":
			json.NewEncoder(w).Encode(map[string]any{
				"MediaContainer": map[string]any{
					"Directory": []map[string]any{{"key": "9"}},
				},
			})
		case "/library/sections/9/refresh":
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer plexSrv.Close()

	parsed, err := url.Parse(plexSrv.URL)
	require.NoError(t, err)
	_, port, err := net.SplitHostPort(parsed.Host)
	require.NoError(t, err)

	fakePlexDirect := fmt.Sprintf("http://127-0-0-1.aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa0.plex.direct:%s", port)

	resourcesSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]any{{
			"provides": "server",
			"connections": []map[string]any{
				{"uri": fakePlexDirect, "local": true},
			},
		}})
	}))
	defer resourcesSrv.Close()

	c := newTestClient(plexSrv.Client(), resourcesSrv.URL+"/api/v2/resources")
	c.dnsResolver = func(_ context.Context, host string) ([]string, error) {
		return nil, fmt.Errorf("dns failed for %s", host)
	}

	resolvedURL, _, err := c.resolveServerURL(context.Background(), "tok")
	require.NoError(t, err)
	assert.Equal(t, fakePlexDirect, resolvedURL)
}
