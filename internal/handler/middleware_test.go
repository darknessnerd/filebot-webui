package handler_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/darknessnerd/filebot-webui/internal/domain"
	"github.com/darknessnerd/filebot-webui/internal/handler"
	"github.com/darknessnerd/filebot-webui/internal/logger"
)

// ── stubs ─────────────────────────────────────────────────────────────────────

type stubValidator struct {
	plexID string
	err    error
}

func (s *stubValidator) ValidateJWT(_ string) (string, error) { return s.plexID, s.err }

type stubUserLoader struct {
	user *domain.User
	err  error
}

func (s *stubUserLoader) FindByPlexID(_ context.Context, _ string) (*domain.User, error) {
	return s.user, s.err
}

func sentinelHandler(t *testing.T, called *bool) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		*called = true
		w.WriteHeader(http.StatusOK)
	})
}

func requireAuth(v *stubValidator, l *stubUserLoader) func(http.Handler) http.Handler {
	return handler.RequireAuth(v, l, logger.New("error", false))
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestRequireAuth_NoCookie_RedirectsLogin(t *testing.T) {
	called := false
	mw := requireAuth(&stubValidator{plexID: "x"}, &stubUserLoader{user: &domain.User{}})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	mw(sentinelHandler(t, &called)).ServeHTTP(w, r)

	assert.Equal(t, http.StatusFound, w.Code)
	assert.Contains(t, w.Header().Get("Location"), "/login")
	assert.False(t, called)
}

func TestRequireAuth_EmptyCookieValue_RedirectsLogin(t *testing.T) {
	called := false
	mw := requireAuth(&stubValidator{plexID: "x"}, &stubUserLoader{user: &domain.User{}})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: "auth_token", Value: ""})
	mw(sentinelHandler(t, &called)).ServeHTTP(w, r)

	assert.Equal(t, http.StatusFound, w.Code)
	assert.Contains(t, w.Header().Get("Location"), "/login")
	assert.False(t, called)
}

func TestRequireAuth_InvalidJWT_RedirectsLogin(t *testing.T) {
	called := false
	mw := requireAuth(
		&stubValidator{err: errors.New("bad token")},
		&stubUserLoader{user: &domain.User{}},
	)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: "auth_token", Value: "bad.token.here"})
	mw(sentinelHandler(t, &called)).ServeHTTP(w, r)

	assert.Equal(t, http.StatusFound, w.Code)
	assert.Contains(t, w.Header().Get("Location"), "/login")
	assert.False(t, called)
}

func TestRequireAuth_UserNotInDB_RedirectsLogin(t *testing.T) {
	called := false
	mw := requireAuth(
		&stubValidator{plexID: "ghost"},
		&stubUserLoader{err: errors.New("not found")},
	)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: "auth_token", Value: "valid.token.here"})
	mw(sentinelHandler(t, &called)).ServeHTTP(w, r)

	assert.Equal(t, http.StatusFound, w.Code)
	assert.Contains(t, w.Header().Get("Location"), "/login")
	assert.False(t, called)
}

func TestRequireAuth_ValidToken_CallsNextWithUser(t *testing.T) {
	user := &domain.User{PlexID: "plex1", PlexUsername: "alice"}
	called := false

	var capturedUser *domain.User
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		capturedUser, _ = handler.UserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	mw := requireAuth(
		&stubValidator{plexID: "plex1"},
		&stubUserLoader{user: user},
	)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: "auth_token", Value: "valid.token"})
	mw(next).ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, called)
	assert.Equal(t, user, capturedUser)
}
