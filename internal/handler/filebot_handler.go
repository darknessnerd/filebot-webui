package handler

import (
	"context"
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"os"
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
	hasTMDB   bool
	log       logger.Logger
}

type torrentOutcome struct {
	TorrentID  string
	SourceName string
	Moved      bool
	Deleted    bool
	Failed     bool
	Message    string
}

func NewFileBotHandler(fb fileBotService, del delugeSvc, plex plexSvc, tmpl *template.Template, mediaRoot string, hasTMDB bool, log logger.Logger) *FileBotHandler {
	return &FileBotHandler{fb: fb, del: del, plex: plex, tmpl: tmpl, mediaRoot: mediaRoot, hasTMDB: hasTMDB, log: log}
}

func (h *FileBotHandler) Form(w http.ResponseWriter, r *http.Request) {
	torrentIDs := r.URL.Query()["torrent_ids"]
	user, _ := UserFromContext(r.Context())

	torrents, err := h.del.ListCompleted(r.Context())
	if err != nil {
		h.log.Error().Err(err).Msg("filebot form: list torrents")
		http.Error(w, "failed to fetch torrents", http.StatusBadGateway)
		return
	}

	torrentByID := make(map[string]domain.Torrent, len(torrents))
	for _, t := range torrents {
		torrentByID[t.ID] = t
	}

	var resolvedIDs []string
	var sourcePaths []string
	var missing []string
	for _, id := range torrentIDs {
		t, found := torrentByID[id]
		if !found {
			missing = append(missing, id)
			continue
		}
		resolvedIDs = append(resolvedIDs, id)
		sourcePaths = append(sourcePaths, filepath.Join(t.DownloadPath, t.Name))
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
	defaultDB := "AniDB"
	if h.hasTMDB {
		defaultDB = "TheMovieDB"
	}
	if err := h.tmpl.ExecuteTemplate(w, "filebot_form", map[string]any{
		"TorrentIDs":  resolvedIDs,
		"SourcePaths": sourcePaths,
		"MediaRoot":   h.mediaRoot,
		"HasTMDB":     h.hasTMDB,
		"DefaultDB":   defaultDB,
		"User":        user,
		"Action":    "",
		"DB":        "",
		"Conflict":  "",
		"Query":     "",
		"Output":    "",
		"Recursive": false,
	}); err != nil {
		h.log.Error().Err(err).Msg("filebot form render")
	}
}

func (h *FileBotHandler) Execute(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	torrentIDs := r.Form["torrent_ids"]
	sourcePaths := r.Form["source_paths"]
	if h.hasError(w, sourcePaths, torrentIDs) {
		return
	}

	baseJob := domain.FileBotJob{
		DB:        r.FormValue("db"),
		Action:    r.FormValue("action"),
		Conflict:  r.FormValue("conflict"),
		Filter:    r.FormValue("filter"),
		Query:     r.FormValue("query"),
		Recursive: r.FormValue("recursive") == "true",
		Output:    r.FormValue("output"),
	}
	if !h.hasTMDB && isTMDBDatabase(baseJob.DB) {
		h.log.Warn().Str("db", baseJob.DB).Msg("filebot execute: tmdb db requested but token not configured")
		http.Error(w, "TMDB provider is unavailable: TMDB_ACCESS_TOKEN is not configured", http.StatusBadRequest)
		return
	}

	h.log.Info().
		Str("action", baseJob.Action).
		Str("db", baseJob.DB).
		Int("torrent_count", len(torrentIDs)).
		Msg("filebot: execute request")

	var result domain.FileBotResult
	hadExecError := false
	hadResultErrors := false
	deletedAny := false
	outcomes := make([]torrentOutcome, 0, len(torrentIDs))

	for i := range torrentIDs {
		outcome := torrentOutcome{
			TorrentID:  torrentIDs[i],
			SourceName: filepath.Base(sourcePaths[i]),
		}
		job := baseJob
		job.TorrentIDs = []string{torrentIDs[i]}
		job.SourcePaths = []string{sourcePaths[i]}

		currentResult, execErr := h.fb.Execute(r.Context(), job)
		result.Successes = append(result.Successes, currentResult.Successes...)
		result.Errors = append(result.Errors, currentResult.Errors...)
		if currentResult.RawOutput != "" {
			if result.RawOutput != "" {
				result.RawOutput += "\n"
			}
			result.RawOutput += currentResult.RawOutput
		}

		if execErr != nil {
			if errors.Is(execErr, domain.ErrInvalidArg) {
				http.Error(w, execErr.Error(), http.StatusBadRequest)
				return
			}
			hadExecError = true
			h.log.Error().
				Err(execErr).
				Str("torrent_id", torrentIDs[i]).
				Str("source_path", sourcePaths[i]).
				Msg("FileBotHandler.Execute")
			outcome.Failed = true
			outcome.Message = execErr.Error()
		}

		if len(currentResult.Errors) > 0 {
			hadResultErrors = true
			outcome.Failed = true
			outcome.Message = currentResult.Errors[0]
		}

		if baseJob.Action == "move" && len(currentResult.Errors) == 0 && execErr == nil {
			outcome.Moved = true
			h.removeSourceDir(sourcePaths[i])
			if derr := h.del.DeleteTorrent(r.Context(), torrentIDs[i]); derr != nil {
				h.log.Warn().Err(derr).Str("torrent_id", torrentIDs[i]).Msg("delete torrent failed")
				outcome.Message = derr.Error()
			} else {
				deletedAny = true
				outcome.Deleted = true
				outcome.Message = "moved and deleted"
			}
		} else if baseJob.Action == "move" && outcome.Failed {
			if outcome.Message == "" {
				outcome.Message = "not moved"
			}
		} else if baseJob.Action != "move" {
			outcome.Message = "not moved (" + baseJob.Action + " action)"
		}

		outcomes = append(outcomes, outcome)
	}

	plexRefreshed := false
	if baseJob.Action == "move" && deletedAny {
		user, ok := UserFromContext(r.Context())
		if ok && user.PlexToken != "" {
			if perr := h.plex.RefreshLibraries(r.Context(), user.PlexToken); perr != nil {
				h.log.Warn().Err(perr).Msg("plex refresh failed")
			} else {
				plexRefreshed = true
			}
		}
	}

	if !hadExecError && !hadResultErrors {
		setToast(w, "success", "FileBot complete")
	} else {
		setToast(w, "error", "FileBot failed: one or more torrents failed")
	}

	w.Header().Set("Content-Type", "text/html")
	if err := h.tmpl.ExecuteTemplate(w, "filebot_result", map[string]any{
		"Successes":       result.Successes,
		"Errors":          result.Errors,
		"RawOutput":       result.RawOutput,
		"PlexRefreshed":   plexRefreshed,
		"TorrentOutcomes": outcomes,
	}); err != nil {
		h.log.Error().Err(err).Msg("filebot result render")
	}
}

func (h *FileBotHandler) hasError(w http.ResponseWriter, sourcePaths []string, torrentIDs []string) bool {
	if len(sourcePaths) == 0 {
		h.log.Warn().Msg("filebot execute: no source_paths in request")
		http.Error(w, "no source paths — torrents may have been removed from Deluge", http.StatusUnprocessableEntity)
		return true
	}
	if len(torrentIDs) == 0 {
		h.log.Warn().Msg("filebot execute: no torrent_ids in request")
		http.Error(w, "no torrent ids", http.StatusBadRequest)
		return true
	}
	if len(torrentIDs) != len(sourcePaths) {
		h.log.Warn().
			Int("torrent_ids", len(torrentIDs)).
			Int("source_paths", len(sourcePaths)).
			Msg("filebot execute: mismatched torrent/source counts")
		http.Error(w, "mismatched torrent/source paths", http.StatusBadRequest)
		return true
	}
	return false
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

func isTMDBDatabase(db string) bool {
	return db == "TheMovieDB" || db == "TheMovieDB::TV"
}

// removeSourceDir removes the source path if it is a directory and is now empty
// (or contains only empty subdirectories) after a successful FileBot move.
// Deluge no longer handles data deletion (removeData=false), so we clean up here.
func (h *FileBotHandler) removeSourceDir(sourcePath string) {
	info, err := os.Stat(sourcePath)
	if err != nil || !info.IsDir() {
		return
	}
	if err := removeEmptyDirs(sourcePath); err != nil {
		h.log.Warn().Err(err).Str("path", sourcePath).Msg("filebot: cleanup source dir failed")
	}
}

func removeEmptyDirs(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			sub := filepath.Join(dir, e.Name())
			if err := removeEmptyDirs(sub); err != nil {
				return err
			}
		}
	}
	// Re-read after recursing — subdirs may now be gone.
	entries, err = os.ReadDir(dir)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return os.Remove(dir)
	}
	return nil
}
