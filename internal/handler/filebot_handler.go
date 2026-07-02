package handler

import (
	"context"
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"path/filepath"

	"github.com/darknessnerd/filebot-webui/internal/domain"
	"github.com/darknessnerd/filebot-webui/internal/logger"
)

type fileBotService interface {
	Execute(ctx context.Context, job domain.FileBotJob) (domain.FileBotResult, error)
}

type delugeSvc interface {
	ListCompleted(ctx context.Context) ([]domain.Torrent, error)
	DeleteTorrent(ctx context.Context, id string) error
}

type plexSvc interface {
	RefreshLibraries(ctx context.Context, plexToken string) error
}

type FileBotHandler struct {
	fb        fileBotService
	del       delugeSvc
	plex      plexSvc
	tmpl      *template.Template
	mediaRoot string
	log       logger.Logger
}

func NewFileBotHandler(fb fileBotService, del delugeSvc, plex plexSvc, tmpl *template.Template, mediaRoot string, log logger.Logger) *FileBotHandler {
	return &FileBotHandler{fb: fb, del: del, plex: plex, tmpl: tmpl, mediaRoot: mediaRoot, log: log}
}

func (h *FileBotHandler) Form(w http.ResponseWriter, r *http.Request) {
	torrentIDs := r.URL.Query()["torrent_ids"]
	user, _ := UserFromContext(r.Context())

	idSet := make(map[string]bool, len(torrentIDs))
	for _, id := range torrentIDs {
		idSet[id] = true
	}

	torrents, err := h.del.ListCompleted(r.Context())
	if err != nil {
		h.log.Error().Err(err).Msg("filebot form: list torrents")
		http.Error(w, "failed to fetch torrents", http.StatusBadGateway)
		return
	}

	var sourcePaths []string
	var missing []string
	for id := range idSet {
		found := false
		for _, t := range torrents {
			if t.ID == id {
				sourcePaths = append(sourcePaths, filepath.Join(t.DownloadPath, t.Name))
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, id)
		}
	}

	if len(missing) > 0 {
		h.log.Warn().Strs("missing_ids", missing).Msg("filebot form: torrent IDs not found in Deluge")
	}

	if len(sourcePaths) == 0 {
		h.log.Warn().Strs("requested_ids", torrentIDs).Msg("filebot form: no torrents resolved — all missing from Deluge")
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusUnprocessableEntity)
		if err := h.tmpl.ExecuteTemplate(w, "filebot_not_found", nil); err != nil {
			h.log.Error().Err(err).Msg("filebot not_found render")
		}
		return
	}

	w.Header().Set("Content-Type", "text/html")
	if err := h.tmpl.ExecuteTemplate(w, "filebot_form", map[string]any{
		"TorrentIDs":  torrentIDs,
		"SourcePaths": sourcePaths,
		"MediaRoot":   h.mediaRoot,
		"User":        user,
		"Action":      "",
		"DB":          "",
		"Conflict":    "",
		"LogLevel":    "",
		"Format":      "",
		"Output":      "",
		"Recursive":   false,
	}); err != nil {
		h.log.Error().Err(err).Msg("filebot form render")
	}
}

func (h *FileBotHandler) Execute(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if len(r.Form["source_paths"]) == 0 {
		h.log.Warn().Msg("filebot execute: no source_paths in request")
		http.Error(w, "no source paths — torrents may have been removed from Deluge", http.StatusUnprocessableEntity)
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

	h.log.Info().
		Str("action", job.Action).
		Str("db", job.DB).
		Int("torrent_count", len(job.TorrentIDs)).
		Msg("filebot: execute request")

	result, err := h.fb.Execute(r.Context(), job)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidArg) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		h.log.Error().Err(err).Msg("FileBotHandler.Execute")
		setToast(w, "error", "FileBot failed: "+err.Error())
	}

	plexRefreshed := false
	if job.Action == "move" && len(result.Errors) == 0 && err == nil {
		for _, id := range job.TorrentIDs {
			if derr := h.del.DeleteTorrent(r.Context(), id); derr != nil {
				h.log.Warn().Err(derr).Str("torrent_id", id).Msg("delete torrent failed")
			}
		}

		user, ok := UserFromContext(r.Context())
		if ok && user.PlexToken != "" {
			if perr := h.plex.RefreshLibraries(r.Context(), user.PlexToken); perr != nil {
				h.log.Warn().Err(perr).Msg("plex refresh failed")
			} else {
				plexRefreshed = true
			}
		}
	}

	if err == nil && len(result.Errors) == 0 {
		setToast(w, "success", "FileBot complete")
	}

	w.Header().Set("Content-Type", "text/html")
	if err := h.tmpl.ExecuteTemplate(w, "filebot_result", map[string]any{
		"Successes":     result.Successes,
		"Errors":        result.Errors,
		"RawOutput":     result.RawOutput,
		"PlexRefreshed": plexRefreshed,
	}); err != nil {
		h.log.Error().Err(err).Msg("filebot result render")
	}
}

// setToast writes an HX-Trigger header so the frontend toast listener fires.
func setToast(w http.ResponseWriter, level, msg string) {
	type toastPayload struct {
		Level string `json:"level"`
		Msg   string `json:"msg"`
	}
	payload, _ := json.Marshal(map[string]toastPayload{
		"showToast": {Level: level, Msg: msg},
	})
	w.Header().Set("HX-Trigger", string(payload))
}
