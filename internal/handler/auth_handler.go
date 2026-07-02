package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/darknessnerd/filebot-webui/internal/domain"
	"github.com/darknessnerd/filebot-webui/internal/logger"
)

type authService interface {
	StartPlexPIN(ctx context.Context, forwardURL string) (authAppURL string, pinID int64, pinCode string, err error)
	CompleteAuth(ctx context.Context, pinID int64, pinCode string) (*domain.User, error)
	IssueJWT(u *domain.User) (string, error)
}

type AuthHandler struct {
	svc         authService
	log         logger.Logger
	redirectURL string
}

func NewAuthHandler(svc authService, redirectURL string, log logger.Logger) *AuthHandler {
	return &AuthHandler{svc: svc, log: log, redirectURL: redirectURL}
}

func (h *AuthHandler) LoginPage(w http.ResponseWriter, r *http.Request) {
	// Sprint 3 will render login.html; placeholder redirect for now.
	http.ServeContent(w, r, "", time.Time{}, nil)
}

func (h *AuthHandler) PlexStart(w http.ResponseWriter, r *http.Request) {
	h.log.Info().Msg("auth: plex PIN flow started")
	authURL, pinID, pinCode, err := h.svc.StartPlexPIN(r.Context(), h.redirectURL)
	if err != nil {
		h.log.Error().Err(err).Msg("PlexStart: StartPlexPIN failed")
		http.Error(w, "failed to start Plex auth", http.StatusInternalServerError)
		return
	}

	ttl := 5 * time.Minute
	secure := isSecure(r)
	http.SetCookie(w, &http.Cookie{
		Name: "plex_pin_id", Value: strconv.FormatInt(pinID, 10),
		Path: "/", MaxAge: int(ttl.Seconds()), HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name: "plex_pin_code", Value: pinCode,
		Path: "/", MaxAge: int(ttl.Seconds()), HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"authAppUrl": authURL})
}

func (h *AuthHandler) PlexForward(w http.ResponseWriter, r *http.Request) {
	pinIDCookie, err := r.Cookie("plex_pin_id")
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	pinCodeCookie, err := r.Cookie("plex_pin_code")
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	pinID, err := strconv.ParseInt(pinIDCookie.Value, 10, 64)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	user, err := h.svc.CompleteAuth(r.Context(), pinID, pinCodeCookie.Value)
	if err != nil {
		h.log.Error().Err(err).Msg("PlexForward: CompleteAuth failed")
		http.Redirect(w, r, "/login?error=auth_failed", http.StatusFound)
		return
	}

	token, err := h.svc.IssueJWT(user)
	if err != nil {
		h.log.Error().Err(err).Msg("PlexForward: IssueJWT failed")
		http.Error(w, "jwt error", http.StatusInternalServerError)
		return
	}

	// Clear PIN cookies.
	secure := isSecure(r)
	for _, name := range []string{"plex_pin_id", "plex_pin_code"} {
		http.SetCookie(w, &http.Cookie{Name: name, Value: "", Path: "/", MaxAge: -1, Secure: secure})
	}

	http.SetCookie(w, &http.Cookie{
		Name: "auth_token", Value: token,
		Path: "/", HttpOnly: true, Secure: isSecure(r), SameSite: http.SameSiteLaxMode,
	})
	h.log.Info().Str("plex_username", user.PlexUsername).Msg("auth: user logged in")
	http.Redirect(w, r, "/", http.StatusFound)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	h.log.Info().Msg("auth: user logged out")
	http.SetCookie(w, &http.Cookie{
		Name: "auth_token", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: isSecure(r),
	})
	http.Redirect(w, r, "/login", http.StatusFound)
}
