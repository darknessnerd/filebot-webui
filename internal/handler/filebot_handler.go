package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"github.com/darknessnerd/filebot-webui/internal/domain"
	"github.com/darknessnerd/filebot-webui/internal/logger"
)

type fileBotService interface {
	Execute(ctx context.Context, job domain.FileBotJob) (domain.FileBotResult, error)
}

type delugeSvc interface {
	DeleteTorrent(ctx context.Context, id string) error
}

type plexSvc interface {
	RefreshLibraries(ctx context.Context, plexToken string) error
}

type FileBotHandler struct {
	fb    fileBotService
	del   delugeSvc
	plex  plexSvc
	log   logger.Logger
}

func NewFileBotHandler(fb fileBotService, del delugeSvc, plex plexSvc, log logger.Logger) *FileBotHandler {
	return &FileBotHandler{fb: fb, del: del, plex: plex, log: log}
}

func (h *FileBotHandler) Form(w http.ResponseWriter, r *http.Request) {
	// Sprint 3 renders filebot_form.html; placeholder for now.
	torrentIDs := r.URL.Query()["torrent_ids"]
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"page": "filebot_form", "torrent_ids": torrentIDs})
}

func (h *FileBotHandler) Execute(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	job := domain.FileBotJob{
		TorrentIDs:  r.Form["torrent_ids"],
		SourcePaths: r.Form["source_paths"],
		DB:          r.FormValue("db"),
		Action:      r.FormValue("action"),
		Conflict:    r.FormValue("conflict"),
		LogLevel:    r.FormValue("log_level"),
		Format:      r.FormValue("format"),
		Filter:      r.FormValue("filter"),
		Query:       r.FormValue("query"),
		Recursive:   r.FormValue("recursive") == "true",
		Output:      r.FormValue("output"),
	}

	result, err := h.fb.Execute(r.Context(), job)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidArg) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		h.log.Error().Err(err).Msg("FileBotHandler.Execute")
		http.Error(w, "filebot execution failed", http.StatusInternalServerError)
		return
	}

	plexRefreshed := false
	if job.Action == "move" && len(result.Errors) == 0 {
		for _, id := range job.TorrentIDs {
			if err := h.del.DeleteTorrent(r.Context(), id); err != nil {
				h.log.Warn().Err(err).Str("torrent_id", id).Msg("delete torrent failed")
			}
		}

		user, ok := UserFromContext(r.Context())
		if ok && user.PlexToken != "" {
			if err := h.plex.RefreshLibraries(r.Context(), user.PlexToken); err != nil {
				h.log.Warn().Err(err).Msg("plex refresh failed")
			} else {
				plexRefreshed = true
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"successes":      result.Successes,
		"errors":         result.Errors,
		"raw_output":     result.RawOutput,
		"plex_refreshed": plexRefreshed,
	})
}
