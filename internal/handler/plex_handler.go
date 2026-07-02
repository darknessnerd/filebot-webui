package handler

import (
	"context"
	"net/http"

	"github.com/darknessnerd/filebot-webui/internal/logger"
)

type plexRefreshSvc interface {
	RefreshLibraries(ctx context.Context, plexToken string) error
}

type PlexHandler struct {
	plex plexRefreshSvc
	log  logger.Logger
}

func NewPlexHandler(plex plexRefreshSvc, log logger.Logger) *PlexHandler {
	return &PlexHandler{plex: plex, log: log}
}

func (h *PlexHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok || user.PlexToken == "" {
		http.Error(w, "no plex token", http.StatusUnauthorized)
		return
	}

	if err := h.plex.RefreshLibraries(r.Context(), user.PlexToken); err != nil {
		h.log.Warn().Err(err).Msg("plex refresh failed")
		setToast(w, "error", "Plex refresh failed: "+err.Error())
		w.WriteHeader(http.StatusBadGateway)
		return
	}

	setToast(w, "success", "Plex libraries refreshed")
	w.WriteHeader(http.StatusNoContent)
}
