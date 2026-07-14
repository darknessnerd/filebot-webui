package handler_test

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
{{define "filebot_form"}}FORM:{{range .TorrentIDs}}{{.}},{{end}}PATHS:{{range .SourcePaths}}{{.}},{{end}}{{end}}
{{define "filebot_result"}}RESULT:{{range .Successes}}OK{{end}}{{range .Errors}}ERR{{end}}:plex={{.PlexRefreshed}}{{end}}
{{define "filebot_not_found"}}NOT_FOUND{{end}}
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

// TestFileBotExecute_Exit3Success_MoveTriggersDeleteAndRefresh guards the regression:
// executor returns (result{Successes}, nil) for exit 3 on move — handler must treat
// this identically to exit-0 success and proceed with delete + Plex refresh.
func TestFileBotExecute_Exit3Success_MoveTriggersDeleteAndRefresh(t *testing.T) {
	fb := &stubFBSvc{result: domain.FileBotResult{Successes: []string{"No input files"}}}
	del := &captureDelSvc{called: new(bool)}
	px := &capturePlexSvc{called: new(bool)}

	h := handler.NewFileBotHandler(fb, del, px,
		fbTmpl(t), "/media", logger.New("error", false))

	w := httptest.NewRecorder()
	r := postForm(validForm()) // action=move
	user := &domain.User{PlexToken: "tok"}
	r = r.WithContext(handler.WithUser(r.Context(), user))
	h.Execute(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, *del.called, "DeleteTorrent must be called when exit 3 treated as success")
	assert.True(t, *px.called, "RefreshLibraries must be called when exit 3 treated as success")
	assert.Contains(t, w.Header().Get("HX-Trigger"), "success")
}

// TestFileBotExecute_Exit3Error_NonMove_SkipsDeleteAndRefresh verifies that when
// executor returns an error (exit 3 on non-move action), handler skips cleanup.
func TestFileBotExecute_Exit3Error_NonMove_SkipsDeleteAndRefresh(t *testing.T) {
	fb := &stubFBSvc{
		result: domain.FileBotResult{Errors: []string{"filebot exited with error: exit status 3"}},
		err:    fmt.Errorf("wrap: %w", domain.ErrFileBotFailed),
	}
	del := &captureDelSvc{called: new(bool)}
	px := &capturePlexSvc{called: new(bool)}

	h := handler.NewFileBotHandler(fb, del, px,
		fbTmpl(t), "/media", logger.New("error", false))

	fields := validForm()
	fields["action"] = []string{"copy"}
	w := httptest.NewRecorder()
	r := postForm(fields)
	h.Execute(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.False(t, *del.called, "DeleteTorrent must not be called on executor error")
	assert.False(t, *px.called, "RefreshLibraries must not be called on executor error")
	assert.Contains(t, w.Header().Get("HX-Trigger"), "error")
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

// ── Form: missing torrents ─────────────────────────────────────────

func TestFileBotForm_AllIDsMissingFromDeluge_Returns422NotFound(t *testing.T) {
	// Deluge returns empty list — all requested IDs are gone.
	del := &stubDelSvc{torrents: []domain.Torrent{}}
	h := handler.NewFileBotHandler(
		&stubFBSvc{}, del, &stubPlexSvc{},
		fbTmpl(t), "/media", logger.New("error", false),
	)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/filebot?torrent_ids=gone1&torrent_ids=gone2", nil)
	h.Form(w, r)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.Contains(t, w.Body.String(), "NOT_FOUND")
}

func TestFileBotForm_SomeIDsMissingFromDeluge_RendersOnlyFoundPaths(t *testing.T) {
	// Deluge has "present" but not "gone".
	del := &stubDelSvc{torrents: []domain.Torrent{
		{ID: "present", Name: "Movie.mkv", DownloadPath: "/downloads"},
	}}
	h := handler.NewFileBotHandler(
		&stubFBSvc{}, del, &stubPlexSvc{},
		fbTmpl(t), "/media", logger.New("error", false),
	)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/filebot?torrent_ids=present&torrent_ids=gone", nil)
	h.Form(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "/downloads/Movie.mkv", "found torrent path must appear")
	// PATHS section must not contain the missing ID — extract it and check.
	pathsIdx := strings.Index(body, "PATHS:")
	require.NotEqual(t, -1, pathsIdx, "template must render PATHS section")
	pathsSection := body[pathsIdx:]
	assert.NotContains(t, pathsSection, "gone", "missing ID must not appear in source paths")
}

func TestFileBotForm_DelugeError_Returns502(t *testing.T) {
	del := &stubDelSvc{err: errors.New("deluge unreachable")}
	h := handler.NewFileBotHandler(
		&stubFBSvc{}, del, &stubPlexSvc{},
		fbTmpl(t), "/media", logger.New("error", false),
	)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/filebot?torrent_ids=abc", nil)
	h.Form(w, r)

	assert.Equal(t, http.StatusBadGateway, w.Code)
}

// ── Execute: empty source_paths guard ─────────────────────────────

func TestFileBotExecute_EmptySourcePaths_Returns422(t *testing.T) {
	h := newFBHandler(&stubFBSvc{}, &stubDelSvc{}, &stubPlexSvc{})

	fields := validForm()
	delete(fields, "source_paths")

	w := httptest.NewRecorder()
	r := postForm(fields)
	h.Execute(w, r)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

// ── Execute: HX-Trigger toast header ──────────────────────────────

func TestFileBotExecute_Success_SetsSuccessToastHeader(t *testing.T) {
	fb := &stubFBSvc{result: domain.FileBotResult{Successes: []string{"ok"}}}
	h := newFBHandler(fb, &stubDelSvc{}, &stubPlexSvc{})

	w := httptest.NewRecorder()
	r := postForm(validForm())
	h.Execute(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	hxTrigger := w.Header().Get("HX-Trigger")
	assert.Contains(t, hxTrigger, "showToast")
	assert.Contains(t, hxTrigger, "success")
}

func TestFileBotExecute_FileBotFailed_SetsErrorToastHeader(t *testing.T) {
	fb := &stubFBSvc{
		result: domain.FileBotResult{Errors: []string{"rename failed"}},
		err:    fmt.Errorf("wrap: %w", domain.ErrFileBotFailed),
	}
	h := newFBHandler(fb, &stubDelSvc{}, &stubPlexSvc{})

	w := httptest.NewRecorder()
	r := postForm(validForm())
	h.Execute(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	hxTrigger := w.Header().Get("HX-Trigger")
	assert.Contains(t, hxTrigger, "showToast")
	assert.Contains(t, hxTrigger, "error")
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
