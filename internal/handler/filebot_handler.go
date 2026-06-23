package handler

import (
	"context"
	"errors"
	"html/template"
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
	w.Header().Set("Content-Type", "text/html")
	if err := h.tmpl.ExecuteTemplate(w, "filebot_form", map[string]any{
		"TorrentIDs": torrentIDs,
		"MediaRoot":  h.mediaRoot,
		"User":       user,
		"Action":     "",
		"DB":         "",
		"Conflict":   "",
		"LogLevel":   "",
		"Format":     "",
		"Output":     "",
		"Recursive":  false,
	}); err != nil {
		h.log.Error().Err(err).Msg("filebot form render")
	}
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
