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

// ── stubs ─────────────────────────────────────────────────────────

type stubAuthSvc struct {
	startURL   string
	startPinID int64
	startCode  string
	startErr   error

	completeUser *domain.User
	completeErr  error

	jwtToken string
	jwtErr   error
}

func (s *stubAuthSvc) StartPlexPIN(_ context.Context, _ string) (string, int64, string, error) {
	return s.startURL, s.startPinID, s.startCode, s.startErr
}
func (s *stubAuthSvc) CompleteAuth(_ context.Context, _ int64, _ string) (*domain.User, error) {
	return s.completeUser, s.completeErr
}
func (s *stubAuthSvc) IssueJWT(_ *domain.User) (string, error) {
	return s.jwtToken, s.jwtErr
}

func newAuthHandler(svc *stubAuthSvc) *handler.AuthHandler {
	return handler.NewAuthHandler(svc, "http://localhost/auth/plex/forward", logger.New("error", false))
}

// ── PlexStart ─────────────────────────────────────────────────────

func TestPlexStart_Happy(t *testing.T) {
	svc := &stubAuthSvc{
		startURL: "https://app.plex.tv/auth#?clientID=x&code=abc",
		startPinID: 42,
		startCode:  "abc",
	}
	h := newAuthHandler(svc)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/auth/plex/start", nil)
	h.PlexStart(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var body map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	assert.Equal(t, svc.startURL, body["authAppUrl"])

	cookies := w.Result().Cookies()
	names := map[string]string{}
	for _, c := range cookies {
		names[c.Name] = c.Value
	}
	assert.Equal(t, "42", names["plex_pin_id"])
	assert.Equal(t, "abc", names["plex_pin_code"])
}

func TestPlexStart_ServiceError(t *testing.T) {
	svc := &stubAuthSvc{startErr: errors.New("plex down")}
	h := newAuthHandler(svc)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/auth/plex/start", nil)
	h.PlexStart(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// ── PlexForward ───────────────────────────────────────────────────

func TestPlexForward_NoCookies_RedirectsLogin(t *testing.T) {
	h := newAuthHandler(&stubAuthSvc{})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/auth/plex/forward", nil)
	h.PlexForward(w, r)

	assert.Equal(t, http.StatusFound, w.Code)
	assert.Contains(t, w.Header().Get("Location"), "/login")
}

func TestPlexForward_Happy(t *testing.T) {
	svc := &stubAuthSvc{
		completeUser: &domain.User{PlexID: "plex1", PlexUsername: "alice"},
		jwtToken:     "tok.tok.tok",
	}
	h := newAuthHandler(svc)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/auth/plex/forward", nil)
	r.AddCookie(&http.Cookie{Name: "plex_pin_id", Value: "99"})
	r.AddCookie(&http.Cookie{Name: "plex_pin_code", Value: "xyz"})
	h.PlexForward(w, r)

	assert.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "/", w.Header().Get("Location"))

	var authToken string
	for _, c := range w.Result().Cookies() {
		if c.Name == "auth_token" {
			authToken = c.Value
		}
	}
	assert.Equal(t, "tok.tok.tok", authToken)
}

func TestPlexForward_CompleteAuthError_RedirectsLogin(t *testing.T) {
	svc := &stubAuthSvc{completeErr: errors.New("pin expired")}
	h := newAuthHandler(svc)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/auth/plex/forward", nil)
	r.AddCookie(&http.Cookie{Name: "plex_pin_id", Value: "1"})
	r.AddCookie(&http.Cookie{Name: "plex_pin_code", Value: "code"})
	h.PlexForward(w, r)

	assert.Equal(t, http.StatusFound, w.Code)
	assert.Contains(t, w.Header().Get("Location"), "/login")
}

// ── Logout ────────────────────────────────────────────────────────

func TestLogout_ClearsCookieAndRedirects(t *testing.T) {
	h := newAuthHandler(&stubAuthSvc{})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	r.AddCookie(&http.Cookie{Name: "auth_token", Value: "some.token"})
	h.Logout(w, r)

	assert.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "/login", w.Header().Get("Location"))

	for _, c := range w.Result().Cookies() {
		if c.Name == "auth_token" {
			assert.Equal(t, -1, c.MaxAge)
		}
	}
}
