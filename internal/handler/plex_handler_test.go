package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/darknessnerd/filebot-webui/internal/domain"
	"github.com/darknessnerd/filebot-webui/internal/handler"
	"github.com/darknessnerd/filebot-webui/internal/logger"
)

// ── stubs ──────────────────────────────────────────────────────────

type stubPlexRefreshSvc struct {
	err error
}

func (s *stubPlexRefreshSvc) RefreshLibraries(_ context.Context, _ string) error {
	return s.err
}

// ── helpers ─────────────────────────────────────────────────────────

func newPlexRefreshRequest(t *testing.T, user *domain.User) *http.Request {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "/plex/refresh", nil)
	if user != nil {
		r = r.WithContext(handler.WithUser(r.Context(), user))
	}
	return r
}

func hxTrigger(t *testing.T, w *httptest.ResponseRecorder) map[string]struct {
	Level string `json:"level"`
	Msg   string `json:"msg"`
} {
	t.Helper()
	hdr := w.Header().Get("HX-Trigger")
	if hdr == "" {
		return nil
	}
	var out map[string]struct {
		Level string `json:"level"`
		Msg   string `json:"msg"`
	}
	require.NoError(t, json.Unmarshal([]byte(hdr), &out))
	return out
}

// ── Tests ────────────────────────────────────────────────────────────

func TestPlexRefresh_Happy(t *testing.T) {
	svc := &stubPlexRefreshSvc{}
	h := handler.NewPlexHandler(svc, logger.New("error", false))

	user := &domain.User{PlexToken: "tok123"}
	w := httptest.NewRecorder()
	h.Refresh(w, newPlexRefreshRequest(t, user))

	assert.Equal(t, http.StatusNoContent, w.Code)
	toast := hxTrigger(t, w)
	require.NotNil(t, toast)
	assert.Equal(t, "success", toast["showToast"].Level)
}

func TestPlexRefresh_NoUser_401(t *testing.T) {
	svc := &stubPlexRefreshSvc{}
	h := handler.NewPlexHandler(svc, logger.New("error", false))

	w := httptest.NewRecorder()
	h.Refresh(w, newPlexRefreshRequest(t, nil))

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestPlexRefresh_EmptyToken_401(t *testing.T) {
	svc := &stubPlexRefreshSvc{}
	h := handler.NewPlexHandler(svc, logger.New("error", false))

	user := &domain.User{PlexToken: ""}
	w := httptest.NewRecorder()
	h.Refresh(w, newPlexRefreshRequest(t, user))

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestPlexRefresh_ServiceError_502(t *testing.T) {
	svc := &stubPlexRefreshSvc{err: errors.New("plex unreachable")}
	h := handler.NewPlexHandler(svc, logger.New("error", false))

	user := &domain.User{PlexToken: "tok123"}
	w := httptest.NewRecorder()
	h.Refresh(w, newPlexRefreshRequest(t, user))

	assert.Equal(t, http.StatusBadGateway, w.Code)
	toast := hxTrigger(t, w)
	require.NotNil(t, toast)
	assert.Equal(t, "error", toast["showToast"].Level)
	assert.Contains(t, toast["showToast"].Msg, "plex unreachable")
}
