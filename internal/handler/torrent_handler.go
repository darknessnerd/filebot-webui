package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/darknessnerd/filebot-webui/internal/domain"
	"github.com/darknessnerd/filebot-webui/internal/logger"
)

type torrentService interface {
	ListCompleted(ctx context.Context) ([]domain.Torrent, error)
}

type TorrentHandler struct {
	svc torrentService
	log logger.Logger
}

func NewTorrentHandler(svc torrentService, log logger.Logger) *TorrentHandler {
	return &TorrentHandler{svc: svc, log: log}
}

func (h *TorrentHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	// Sprint 3 renders dashboard.html; placeholder JSON for now.
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"page": "dashboard"})
}

func (h *TorrentHandler) List(w http.ResponseWriter, r *http.Request) {
	torrents, err := h.svc.ListCompleted(r.Context())
	if err != nil {
		h.log.Error().Err(err).Msg("TorrentHandler.List")
		http.Error(w, "failed to fetch torrents", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(torrents)
}
