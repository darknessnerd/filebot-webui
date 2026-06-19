package handler_test

import (
	"context"
	"errors"
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/darknessnerd/filebot-webui/internal/domain"
	"github.com/darknessnerd/filebot-webui/internal/handler"
	"github.com/darknessnerd/filebot-webui/internal/logger"
)

// ── stubs ──────────────────────────────────────────────────────────

type stubTorrentSvc struct {
	torrents []domain.Torrent
	err      error
}

func (s *stubTorrentSvc) ListCompleted(_ context.Context) ([]domain.Torrent, error) {
	return s.torrents, s.err
}

func minimalTmpl(t *testing.T) *template.Template {
	t.Helper()
	tmpl, err := template.New("").Funcs(template.FuncMap{
		"formatBytes": func(b int64) string { return "1 KB" },
		"or":          func(a, b string) string { if a != "" { return a }; return b },
		"list":        func(args ...string) []string { return args },
	}).Parse(`
{{define "base"}}<html><body>{{block "content" .}}{{end}}</body></html>{{end}}
{{define "torrents"}}{{if .Error}}<div class="error">{{.Error}}</div>{{else}}{{range .Torrents}}<tr>{{.Name}}</tr>{{end}}{{end}}{{end}}
`)
	require.NoError(t, err)
	return tmpl
}

// ── Tests ──────────────────────────────────────────────────────────

func TestTorrentList_Happy(t *testing.T) {
	svc := &stubTorrentSvc{torrents: []domain.Torrent{{ID: "a1", Name: "Movie.mkv"}}}
	tmpl := minimalTmpl(t)
	h := handler.NewTorrentHandler(svc, tmpl, logger.New("error", false))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/torrents", nil)
	h.List(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
	assert.Contains(t, w.Body.String(), "Movie.mkv")
}

func TestTorrentList_ServiceError_RendersErrorPartial(t *testing.T) {
	svc := &stubTorrentSvc{err: errors.New("deluge down")}
	tmpl := minimalTmpl(t)
	h := handler.NewTorrentHandler(svc, tmpl, logger.New("error", false))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/torrents", nil)
	h.List(w, r)

	// Still 200 — renders error partial, not 500
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "error")
}

func TestTorrentDashboard_RequiresNoUser(t *testing.T) {
	svc := &stubTorrentSvc{}
	tmpl := minimalTmpl(t)
	h := handler.NewTorrentHandler(svc, tmpl, logger.New("error", false))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	h.Dashboard(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, strings.ToLower(w.Body.String()), "html")
}
