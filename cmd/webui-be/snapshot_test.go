package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/darknessnerd/filebot-webui/internal/domain"
	"github.com/darknessnerd/filebot-webui/internal/handler"
	"github.com/darknessnerd/filebot-webui/internal/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Regenerate with: go test ./cmd/webui-be -run TestSnapshot -update
var updateSnapshots = flag.Bool("update", false, "rewrite golden snapshot files")

var snapshotUser = &domain.User{ID: 1, PlexID: "plex-1", PlexUsername: "snapshot-user", PlexToken: "tok"}

var snapshotTorrents = []domain.Torrent{
	{ID: "abc123", Name: "The.Matrix.1999.1080p.BluRay", State: "Seeding", Progress: 100, DownloadPath: "/downloads", Size: 8_589_934_592, IsFinished: true},
	{ID: "def456", Name: "Breaking.Bad.S01E01.1080p", State: "Downloading", Progress: 42.4, DownloadPath: "/downloads", Size: 2_147_483_648},
}

type snapshotDeluge struct {
	torrents []domain.Torrent
	err      error
}

func (s *snapshotDeluge) ListCompleted(context.Context) ([]domain.Torrent, error) {
	return s.torrents, s.err
}

func (s *snapshotDeluge) DeleteTorrent(context.Context, string) error { return nil }

type snapshotPlex struct{}

func (snapshotPlex) RefreshLibraries(context.Context, string) error { return nil }

type snapshotFileBot struct {
	result domain.FileBotResult
	err    error
}

func (s *snapshotFileBot) Execute(context.Context, domain.FileBotJob) (domain.FileBotResult, error) {
	return s.result, s.err
}

func TestSnapshot(t *testing.T) {
	tmpl, err := parseTemplates(false)
	require.NoError(t, err)
	log := logger.New("disabled", false)

	torrentH := func(del *snapshotDeluge) *handler.TorrentHandler {
		return handler.NewTorrentHandler(del, tmpl, log, false)
	}
	fileBotH := func(fb *snapshotFileBot, hasTMDB bool) *handler.FileBotHandler {
		return handler.NewFileBotHandler(fb, &snapshotDeluge{torrents: snapshotTorrents}, snapshotPlex{}, tmpl, "/media", hasTMDB, log)
	}
	executeForm := url.Values{
		"torrent_ids":  {"abc123"},
		"source_paths": {"/downloads/The.Matrix.1999.1080p.BluRay"},
		"db":           {"TheMovieDB"},
		"action":       {"copy"},
		"conflict":     {"skip"},
		"output":       {"/media"},
	}

	cases := []struct {
		name       string
		req        *http.Request
		handler    http.HandlerFunc
		wantStatus int
	}{
		{
			name:       "dashboard",
			req:        httptest.NewRequest(http.MethodGet, "/", nil),
			handler:    torrentH(&snapshotDeluge{}).Dashboard,
			wantStatus: http.StatusOK,
		},
		{
			name:       "torrents_list",
			req:        httptest.NewRequest(http.MethodGet, "/torrents", nil),
			handler:    torrentH(&snapshotDeluge{torrents: snapshotTorrents}).List,
			wantStatus: http.StatusOK,
		},
		{
			name:       "torrents_empty",
			req:        httptest.NewRequest(http.MethodGet, "/torrents", nil),
			handler:    torrentH(&snapshotDeluge{}).List,
			wantStatus: http.StatusOK,
		},
		{
			name:       "torrents_error",
			req:        httptest.NewRequest(http.MethodGet, "/torrents", nil),
			handler:    torrentH(&snapshotDeluge{err: errors.New("deluge down")}).List,
			wantStatus: http.StatusOK,
		},
		{
			name:       "filebot_form_tmdb",
			req:        httptest.NewRequest(http.MethodGet, "/filebot?torrent_ids=abc123&torrent_ids=def456", nil),
			handler:    fileBotH(&snapshotFileBot{}, true).Form,
			wantStatus: http.StatusOK,
		},
		{
			name:       "filebot_form_no_tmdb",
			req:        httptest.NewRequest(http.MethodGet, "/filebot?torrent_ids=abc123", nil),
			handler:    fileBotH(&snapshotFileBot{}, false).Form,
			wantStatus: http.StatusOK,
		},
		{
			name:       "filebot_not_found",
			req:        httptest.NewRequest(http.MethodGet, "/filebot?torrent_ids=missing", nil),
			handler:    fileBotH(&snapshotFileBot{}, true).Form,
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name: "filebot_result_success",
			req:  formRequest(executeForm),
			handler: fileBotH(&snapshotFileBot{result: domain.FileBotResult{
				Successes: []string{"/media/Movies/The Matrix (1999)/The Matrix (1999).mkv"},
				RawOutput: "[COPY] from [/downloads/The.Matrix.1999.1080p.BluRay/matrix.mkv] to [/media/Movies/The Matrix (1999)/The Matrix (1999).mkv]",
			}}, true).Execute,
			wantStatus: http.StatusOK,
		},
		{
			name: "filebot_result_error",
			req:  formRequest(executeForm),
			handler: fileBotH(&snapshotFileBot{
				result: domain.FileBotResult{Errors: []string{"no video files found in selected source paths"}},
				err:    domain.ErrFileBotFailed,
			}, true).Execute,
			wantStatus: http.StatusOK,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			tc.handler(w, tc.req.WithContext(handler.WithUser(tc.req.Context(), snapshotUser)))
			require.Equal(t, tc.wantStatus, w.Code)
			assertGolden(t, tc.name, w.Body.Bytes())
		})
	}

	t.Run("login", func(t *testing.T) {
		data, err := webFS.ReadFile("web/templates/login.html")
		require.NoError(t, err)
		assertGolden(t, "login", data)
	})
}

func formRequest(form url.Values) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/filebot/execute", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return r
}

func assertGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	got = normalizeHTML(got)
	path := filepath.Join("testdata", "snapshots", name+".html")
	if *updateSnapshots {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, got, 0o644))
		return
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err, "missing snapshot %s — run: go test ./cmd/webui-be -run TestSnapshot -update", path)
	assert.Equal(t, string(want), string(got), "snapshot %s differs — if intended, rerun with -update", path)
}

// normalizeHTML drops trailing whitespace and blank lines so template indentation tweaks don't churn snapshots.
func normalizeHTML(b []byte) []byte {
	var out bytes.Buffer
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimRight(line, " \t\r")
		if line == "" {
			continue
		}
		out.WriteString(line)
		out.WriteByte('\n')
	}
	return out.Bytes()
}
