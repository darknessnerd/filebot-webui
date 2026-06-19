package handler

import (
	"context"
	"html/template"
	"net/http"

	"github.com/darknessnerd/filebot-webui/internal/domain"
	"github.com/darknessnerd/filebot-webui/internal/logger"
)

type torrentService interface {
	ListCompleted(ctx context.Context) ([]domain.Torrent, error)
}

type TorrentHandler struct {
	svc  torrentService
	tmpl *template.Template
	log  logger.Logger
}

func NewTorrentHandler(svc torrentService, tmpl *template.Template, log logger.Logger) *TorrentHandler {
	return &TorrentHandler{svc: svc, tmpl: tmpl, log: log}
}

func (h *TorrentHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFromContext(r.Context())
	w.Header().Set("Content-Type", "text/html")
	if err := h.tmpl.ExecuteTemplate(w, "base", map[string]any{"User": user}); err != nil {
		h.log.Error().Err(err).Msg("Dashboard render")
	}
}

func (h *TorrentHandler) List(w http.ResponseWriter, r *http.Request) {
	torrents, err := h.svc.ListCompleted(r.Context())
	data := map[string]any{"Torrents": torrents}
	if err != nil {
		h.log.Error().Err(err).Msg("TorrentHandler.List")
		data["Error"] = "Failed to fetch torrents from Deluge."
	}
	w.Header().Set("Content-Type", "text/html")
	if err := h.tmpl.ExecuteTemplate(w, "torrents", data); err != nil {
		h.log.Error().Err(err).Msg("torrents partial render")
	}
}
