package handler_test

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/darknessnerd/filebot-webui/internal/domain"
	"github.com/darknessnerd/filebot-webui/internal/handler"
	"github.com/darknessnerd/filebot-webui/internal/logger"
)

// ── stubs ──────────────────────────────────────────────────────────

type stubFBSvc struct {
	result domain.FileBotResult
	err    error
}

func (s *stubFBSvc) Execute(_ context.Context, _ domain.FileBotJob) (domain.FileBotResult, error) {
	return s.result, s.err
}

type stubDelSvc struct {
	err      error
	torrents []domain.Torrent
}

func (s *stubDelSvc) ListCompleted(_ context.Context) ([]domain.Torrent, error) {
	return s.torrents, s.err
}
func (s *stubDelSvc) DeleteTorrent(_ context.Context, _ string) error { return s.err }

type stubPlexSvc struct{ err error }

func (s *stubPlexSvc) RefreshLibraries(_ context.Context, _ string) error { return s.err }

func fbTmpl(t *testing.T) *template.Template {
	if t != nil {
		t.Helper()
	}
	return getFBTmpl()
}

var sharedFBTmpl *template.Template

func getFBTmpl() *template.Template {
	if sharedFBTmpl != nil {
		return sharedFBTmpl
	}
	tmpl, err := template.New("").Funcs(template.FuncMap{
		"formatBytes": func(b int64) string { return "0 B" },
		"or":          func(a, b string) string { if a != "" { return a }; return b },
		"list":        func(args ...string) []string { return args },
	}).Parse(`
{{define "filebot_form"}}FORM:{{range .TorrentIDs}}{{.}},{{end}}{{end}}
{{define "filebot_result"}}RESULT:{{range .Successes}}OK{{end}}{{range .Errors}}ERR{{end}}:plex={{.PlexRefreshed}}{{end}}
`)
	if err != nil {
		panic(err)
	}
	sharedFBTmpl = tmpl
	return tmpl
}

func newFBHandler(fb *stubFBSvc, del *stubDelSvc, px *stubPlexSvc) *handler.FileBotHandler {
	return handler.NewFileBotHandler(fb, del, px, getFBTmpl(), "/media", logger.New("error", false))
}

// ── Form ───────────────────────────────────────────────────────────

func TestFileBotForm_PassesTorrentIDs(t *testing.T) {
	del := &stubDelSvc{torrents: []domain.Torrent{
		{ID: "abc", Name: "Movie.A", DownloadPath: "/downloads"},
		{ID: "def", Name: "Movie.B", DownloadPath: "/downloads"},
	}}
	h := handler.NewFileBotHandler(
		&stubFBSvc{}, del, &stubPlexSvc{},
		fbTmpl(t), "/media", logger.New("error", false),
	)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/filebot?torrent_ids=abc&torrent_ids=def", nil)
	h.Form(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "abc")
	assert.Contains(t, body, "def")
}

// ── Execute ────────────────────────────────────────────────────────

func postForm(fields map[string][]string) *http.Request {
	form := url.Values(fields)
	r := httptest.NewRequest(http.MethodPost, "/filebot/execute",
		strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return r
}

func validForm() map[string][]string {
	return map[string][]string{
		"db":          {"TheMovieDB"},
		"action":      {"move"},
		"conflict":    {"skip"},
		"log_level":   {"info"},
		"output":      {"/media/movies"},
		"torrent_ids": {"t1"},
		"source_paths": {"/downloads/file.mkv"},
	}
}

func TestFileBotExecute_Happy_RendersSuccesses(t *testing.T) {
	fb := &stubFBSvc{result: domain.FileBotResult{Successes: []string{"renamed ok"}}}
	h := handler.NewFileBotHandler(fb, &stubDelSvc{}, &stubPlexSvc{},
		fbTmpl(t), "/media", logger.New("error", false))

	w := httptest.NewRecorder()
	r := postForm(validForm())
	h.Execute(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "OK")
}

func TestFileBotExecute_InvalidArg_Returns400(t *testing.T) {
	fb := &stubFBSvc{err: fmt.Errorf("wrap: %w", domain.ErrInvalidArg)}
	h := newFBHandler(fb, &stubDelSvc{}, &stubPlexSvc{})

	w := httptest.NewRecorder()
	r := postForm(validForm())
	h.Execute(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestFileBotExecute_FileBotError_RendersErrorsIn200(t *testing.T) {
	fb := &stubFBSvc{
		result: domain.FileBotResult{Errors: []string{"rename failed"}},
		err:    fmt.Errorf("wrap: %w", domain.ErrFileBotFailed),
	}
	h := newFBHandler(fb, &stubDelSvc{}, &stubPlexSvc{})

	w := httptest.NewRecorder()
	r := postForm(validForm())
	h.Execute(w, r)

	// Non-ErrInvalidArg: handler logs and renders result partial with errors
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "ERR")
}

func TestFileBotExecute_MoveSuccess_DeletesTorrentAndRefreshPlex(t *testing.T) {
	fb := &stubFBSvc{result: domain.FileBotResult{Successes: []string{"ok"}}}
	delWrapper := &captureDelSvc{called: new(bool)}
	pxWrapper := &capturePlexSvc{called: new(bool)}

	h := handler.NewFileBotHandler(fb, delWrapper, pxWrapper,
		fbTmpl(t), "/media", logger.New("error", false))

	// Inject user with PlexToken into context
	w := httptest.NewRecorder()
	r := postForm(validForm())
	user := &domain.User{PlexToken: "tok"}
	r = r.WithContext(handler.WithUser(r.Context(), user))
	h.Execute(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, *delWrapper.called, "DeleteTorrent should have been called")
	assert.True(t, *pxWrapper.called, "RefreshLibraries should have been called")
	assert.Contains(t, w.Body.String(), "plex=true")
}

func TestFileBotExecute_NoMoveAction_SkipsDeleteAndRefresh(t *testing.T) {
	fb := &stubFBSvc{result: domain.FileBotResult{Successes: []string{"ok"}}}
	del := &captureDelSvc{called: new(bool)}
	px := &capturePlexSvc{called: new(bool)}

	h := handler.NewFileBotHandler(fb, del, px,
		fbTmpl(t), "/media", logger.New("error", false))

	fields := validForm()
	fields["action"] = []string{"test"} // not "move"
	w := httptest.NewRecorder()
	r := postForm(fields)
	h.Execute(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.False(t, *del.called, "DeleteTorrent should NOT have been called for action=test")
	assert.False(t, *px.called, "RefreshLibraries should NOT have been called for action=test")
}

// ── capture stubs ──────────────────────────────────────────────────

type captureDelSvc struct{ called *bool }

func (c *captureDelSvc) ListCompleted(_ context.Context) ([]domain.Torrent, error) {
	return nil, nil
}
func (c *captureDelSvc) DeleteTorrent(_ context.Context, _ string) error {
	*c.called = true
	return nil
}

type capturePlexSvc struct{ called *bool }

func (c *capturePlexSvc) RefreshLibraries(_ context.Context, _ string) error {
	*c.called = true
	return nil
}
