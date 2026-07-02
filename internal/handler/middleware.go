package handler

import (
	"context"
	"net/http"

	"github.com/darknessnerd/filebot-webui/internal/domain"
	"github.com/darknessnerd/filebot-webui/internal/logger"
)

// tokenValidator is the subset of AuthService used by the middleware.
type tokenValidator interface {
	ValidateJWT(token string) (plexID string, err error)
}

// userLoader loads a user by Plex ID; used by middleware after JWT validation.
type userLoader interface {
	FindByPlexID(ctx context.Context, plexID string) (*domain.User, error)
}

// RequireAuth returns middleware that validates the auth_token HttpOnly cookie.
// On success it injects *domain.User into the request context.
// On failure it redirects to /login.
func RequireAuth(validator tokenValidator, loader userLoader, log logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("auth_token")
			if err != nil || cookie.Value == "" {
				http.Redirect(w, r, "/login", http.StatusFound)
				return
			}

			plexID, err := validator.ValidateJWT(cookie.Value)
			if err != nil {
				log.Warn().Err(err).Msg("invalid JWT")
				http.Redirect(w, r, "/login", http.StatusFound)
				return
			}

			u, err := loader.FindByPlexID(r.Context(), plexID)
			if err != nil {
				log.Warn().Str("plex_id", plexID).Msg("user not found after JWT validation")
				http.Redirect(w, r, "/login", http.StatusFound)
				return
			}

			log.Debug().Str("plex_id", plexID).Str("path", r.URL.Path).Msg("auth: request authorized")
			next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), u)))
		})
	}
}
