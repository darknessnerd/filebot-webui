package handler_test

// End-to-end error/failure scenario tests: real InternalEngine + real filesystem.
// All external network calls replaced by in-process stubs (same as filebot_e2e_test.go).
//
// Scenario matrix:
//
//	Test                                        DB              Torrent shape               Action  What goes wrong                        Asserts
//	──────────────────────────────────────────  ──────────────  ──────────────────────────  ──────  ─────────────────────────────────────  ─────────────────────────────────────────────────────────
//	Movie_SourceMissing                         TheMovieDB      single flat file (absent)   move    os.Stat fails — source does not exist  error toast; outcome failed; no delete; no plex
//	TV_NoEpisodeMarker                          TheMovieDB::TV  single flat file            move    filename has no SxxExx/NxNN marker     error toast; outcome failed; source untouched; no delete
//	Anime_NoEpisodeMarker                       AniDB           single flat file            move    filename has no episode marker         error toast; outcome failed; source untouched; no delete
//	Movie_MetadataResolverFails                 TheMovieDB      single flat file            move    SearchMovie returns error               error toast; outcome failed; source untouched; no delete
//	TV_MetadataResolverFails                    TheMovieDB::TV  single flat file            move    SearchTV returns error                 error toast; outcome failed; source untouched; no delete
//	Anime_MetadataResolverFails                 AniDB           single flat file            move    SearchAnime returns error              error toast; outcome failed; source untouched; no delete
//	TV_ConflictFail_TargetExists                TheMovieDB::TV  single flat file            move    conflict=fail; target already present  error toast; source untouched; target unchanged; no delete
//	MultipleTorrents_OneFailsOneSucceeds        TheMovieDB::TV  two flat files (2 torrents) move    second file has no episode marker      error toast; t1 deleted; t2 kept; plex refreshed (t1 moved)
//	Anime_NestedFolders_OneSeasonConflictFail   AniDB           folder/S1/ + folder/S2/     move    conflict=fail; S2 target pre-exists    error toast; S1 moved+deleted; S2 outcome failed; no extra delete

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/darknessnerd/filebot-webui/internal/domain"
	"github.com/darknessnerd/filebot-webui/internal/handler"
	"github.com/darknessnerd/filebot-webui/internal/logger"
	"github.com/darknessnerd/filebot-webui/internal/service/filebot"
)

// errMetadata wraps stubMetadata and overrides one resolver method to return an error.
type errMetadata struct {
	stub    *stubMetadata
	failOn  string // "movie", "tv", or "anime"
	failErr error
}

func (e *errMetadata) SearchMovie(ctx context.Context, q string, y int) (*domain.MovieMatch, error) {
	if e.failOn == "movie" {
		return nil, e.failErr
	}
	return e.stub.SearchMovie(ctx, q, y)
}
func (e *errMetadata) SearchTV(ctx context.Context, q string, y int) (*domain.TVMatch, error) {
	if e.failOn == "tv" {
		return nil, e.failErr
	}
	return e.stub.SearchTV(ctx, q, y)
}
func (e *errMetadata) SearchAnime(ctx context.Context, q string, y int) (*domain.AnimeMatch, error) {
	if e.failOn == "anime" {
		return nil, e.failErr
	}
	return e.stub.SearchAnime(ctx, q, y)
}
func (e *errMetadata) SearchAnimeByAID(ctx context.Context, aid int) (*domain.AnimeMatch, error) {
	if e.failOn == "anime" {
		return nil, e.failErr
	}
	return e.stub.SearchAnimeByAID(ctx, aid)
}

func newErrE2EHandler(
	t *testing.T,
	meta filebot.MetadataResolver,
	del *e2eDeluge,
	px *e2ePlex,
	mediaRoot string,
) *handler.FileBotHandler {
	t.Helper()
	log := logger.New("error", false)
	svc := filebot.NewInternal(mediaRoot, meta, log)
	return handler.NewFileBotHandler(svc, del, px, getFBTmpl(), mediaRoot, true, log)
}

// ── source missing ─────────────────────────────────────────────────────────

func TestE2EError_Movie_SourceMissing(t *testing.T) {
	dir := t.TempDir()
	mediaRoot := filepath.Join(dir, "media")
	absent := filepath.Join(dir, "downloads", "Ghost.mkv") // never created

	meta := &stubMetadata{movie: &domain.MovieMatch{ID: 1, Title: "Ghost", Year: 1990}}
	del := &e2eDeluge{}
	px := newE2EPlex()
	h := newErrE2EHandler(t, meta, del, px, mediaRoot)

	w := httptest.NewRecorder()
	r := postForm(map[string][]string{
		"db": {"TheMovieDB"}, "action": {"move"}, "conflict": {"skip"},
		"output": {mediaRoot}, "torrent_ids": {"t1"}, "source_paths": {absent},
	})
	r = r.WithContext(handler.WithUser(r.Context(), &domain.User{PlexToken: "tok"}))
	h.Execute(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("HX-Trigger"), "error")
	assert.Contains(t, w.Body.String(), "t1|false|false|true|")
	assert.Empty(t, del.deletedIDs)
	assert.False(t, px.called)
}

// ── no episode marker ──────────────────────────────────────────────────────

func TestE2EError_TV_NoEpisodeMarker(t *testing.T) {
	dir := t.TempDir()
	mediaRoot := filepath.Join(dir, "media")
	downloadDir := filepath.Join(dir, "downloads")
	require.NoError(t, os.MkdirAll(downloadDir, 0755))

	// Filename with no SxxExx/NxNN pattern.
	src := filepath.Join(downloadDir, "Breaking.Bad.Complete.Season.mkv")
	require.NoError(t, os.WriteFile(src, []byte("data"), 0644))

	meta := &stubMetadata{tv: &domain.TVMatch{ID: 1396, Name: "Breaking Bad", Year: 2008}}
	del := &e2eDeluge{}
	px := newE2EPlex()
	h := newErrE2EHandler(t, meta, del, px, mediaRoot)

	w := httptest.NewRecorder()
	r := postForm(map[string][]string{
		"db": {"TheMovieDB::TV"}, "action": {"move"}, "conflict": {"skip"},
		"output": {mediaRoot}, "torrent_ids": {"t1"}, "source_paths": {src},
	})
	r = r.WithContext(handler.WithUser(r.Context(), &domain.User{PlexToken: "tok"}))
	h.Execute(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("HX-Trigger"), "error")
	assert.Contains(t, w.Body.String(), "t1|false|false|true|")
	assert.FileExists(t, src, "source must be untouched on failure")
	assert.Empty(t, del.deletedIDs)
	assert.False(t, px.called)
}

func TestE2EError_Anime_NoEpisodeMarker(t *testing.T) {
	dir := t.TempDir()
	mediaRoot := filepath.Join(dir, "media")
	downloadDir := filepath.Join(dir, "downloads")
	require.NoError(t, os.MkdirAll(downloadDir, 0755))

	src := filepath.Join(downloadDir, "One.Piece.Complete.Box.Set.mkv")
	require.NoError(t, os.WriteFile(src, []byte("data"), 0644))

	meta := &stubMetadata{anime: &domain.AnimeMatch{ID: 21, Title: "One Piece", Year: 1999}}
	del := &e2eDeluge{}
	px := newE2EPlex()
	h := newErrE2EHandler(t, meta, del, px, mediaRoot)

	w := httptest.NewRecorder()
	r := postForm(map[string][]string{
		"db": {"AniDB"}, "action": {"move"}, "conflict": {"skip"},
		"output": {mediaRoot}, "torrent_ids": {"t1"}, "source_paths": {src},
		"query": {"aid:21"},
	})
	r = r.WithContext(handler.WithUser(r.Context(), &domain.User{PlexToken: "tok"}))
	h.Execute(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("HX-Trigger"), "error")
	assert.Contains(t, w.Body.String(), "t1|false|false|true|")
	assert.FileExists(t, src)
	assert.Empty(t, del.deletedIDs)
	assert.False(t, px.called)
}

// ── metadata resolver errors ───────────────────────────────────────────────

func TestE2EError_Movie_MetadataResolverFails(t *testing.T) {
	dir := t.TempDir()
	mediaRoot := filepath.Join(dir, "media")
	downloadDir := filepath.Join(dir, "downloads")
	require.NoError(t, os.MkdirAll(downloadDir, 0755))

	src := filepath.Join(downloadDir, "Dune.Part.Two.2024.mkv")
	require.NoError(t, os.WriteFile(src, []byte("data"), 0644))

	meta := &errMetadata{
		stub:    &stubMetadata{},
		failOn:  "movie",
		failErr: errors.New("TMDB unavailable"),
	}
	del := &e2eDeluge{}
	px := newE2EPlex()
	h := newErrE2EHandler(t, meta, del, px, mediaRoot)

	w := httptest.NewRecorder()
	r := postForm(map[string][]string{
		"db": {"TheMovieDB"}, "action": {"move"}, "conflict": {"skip"},
		"output": {mediaRoot}, "torrent_ids": {"t1"}, "source_paths": {src},
	})
	r = r.WithContext(handler.WithUser(r.Context(), &domain.User{PlexToken: "tok"}))
	h.Execute(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("HX-Trigger"), "error")
	assert.Contains(t, w.Body.String(), "t1|false|false|true|")
	assert.FileExists(t, src)
	assert.Empty(t, del.deletedIDs)
	assert.False(t, px.called)
}

func TestE2EError_TV_MetadataResolverFails(t *testing.T) {
	dir := t.TempDir()
	mediaRoot := filepath.Join(dir, "media")
	downloadDir := filepath.Join(dir, "downloads")
	require.NoError(t, os.MkdirAll(downloadDir, 0755))

	src := filepath.Join(downloadDir, "Breaking.Bad.S01E01.mkv")
	require.NoError(t, os.WriteFile(src, []byte("data"), 0644))

	meta := &errMetadata{
		stub:    &stubMetadata{},
		failOn:  "tv",
		failErr: errors.New("TMDB rate limited"),
	}
	del := &e2eDeluge{}
	px := newE2EPlex()
	h := newErrE2EHandler(t, meta, del, px, mediaRoot)

	w := httptest.NewRecorder()
	r := postForm(map[string][]string{
		"db": {"TheMovieDB::TV"}, "action": {"move"}, "conflict": {"skip"},
		"output": {mediaRoot}, "torrent_ids": {"t1"}, "source_paths": {src},
	})
	r = r.WithContext(handler.WithUser(r.Context(), &domain.User{PlexToken: "tok"}))
	h.Execute(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("HX-Trigger"), "error")
	assert.Contains(t, w.Body.String(), "t1|false|false|true|")
	assert.FileExists(t, src)
	assert.Empty(t, del.deletedIDs)
	assert.False(t, px.called)
}

func TestE2EError_Anime_MetadataResolverFails(t *testing.T) {
	dir := t.TempDir()
	mediaRoot := filepath.Join(dir, "media")
	downloadDir := filepath.Join(dir, "downloads")
	require.NoError(t, os.MkdirAll(downloadDir, 0755))

	src := filepath.Join(downloadDir, "One.Piece.S01E01.mkv")
	require.NoError(t, os.WriteFile(src, []byte("data"), 0644))

	meta := &errMetadata{
		stub:    &stubMetadata{},
		failOn:  "anime",
		failErr: errors.New("AniDB timeout"),
	}
	del := &e2eDeluge{}
	px := newE2EPlex()
	h := newErrE2EHandler(t, meta, del, px, mediaRoot)

	w := httptest.NewRecorder()
	r := postForm(map[string][]string{
		"db": {"AniDB"}, "action": {"move"}, "conflict": {"skip"},
		"output": {mediaRoot}, "torrent_ids": {"t1"}, "source_paths": {src},
		"query": {"aid:21"},
	})
	r = r.WithContext(handler.WithUser(r.Context(), &domain.User{PlexToken: "tok"}))
	h.Execute(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("HX-Trigger"), "error")
	assert.Contains(t, w.Body.String(), "t1|false|false|true|")
	assert.FileExists(t, src)
	assert.Empty(t, del.deletedIDs)
	assert.False(t, px.called)
}

// ── conflict=fail: target already present ─────────────────────────────────

func TestE2EError_TV_ConflictFail_TargetExists(t *testing.T) {
	dir := t.TempDir()
	mediaRoot := filepath.Join(dir, "media")
	downloadDir := filepath.Join(dir, "downloads")
	require.NoError(t, os.MkdirAll(downloadDir, 0755))

	src := filepath.Join(downloadDir, "Breaking.Bad.S01E01.mkv")
	require.NoError(t, os.WriteFile(src, []byte("source"), 0644))

	// Pre-create the target so conflict=fail triggers.
	target := filepath.Join(mediaRoot, "TV", "Breaking Bad", "Season 1", "Breaking Bad - S01E01.mkv")
	require.NoError(t, os.MkdirAll(filepath.Dir(target), 0755))
	require.NoError(t, os.WriteFile(target, []byte("existing"), 0644))

	meta := &stubMetadata{tv: &domain.TVMatch{ID: 1396, Name: "Breaking Bad", Year: 2008}}
	del := &e2eDeluge{}
	px := newE2EPlex()
	h := newErrE2EHandler(t, meta, del, px, mediaRoot)

	w := httptest.NewRecorder()
	r := postForm(map[string][]string{
		"db": {"TheMovieDB::TV"}, "action": {"move"}, "conflict": {"fail"},
		"output": {mediaRoot}, "torrent_ids": {"t1"}, "source_paths": {src},
	})
	r = r.WithContext(handler.WithUser(r.Context(), &domain.User{PlexToken: "tok"}))
	h.Execute(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("HX-Trigger"), "error")
	assert.Contains(t, w.Body.String(), "t1|false|false|true|")
	assert.FileExists(t, src, "source must be untouched")
	assert.Equal(t, "existing", func() string { b, _ := os.ReadFile(target); return string(b) }(), "target must be unchanged")
	assert.Empty(t, del.deletedIDs)
	assert.False(t, px.called)
}

// ── multi-torrent partial failure ──────────────────────────────────────────

func TestE2EError_MultipleTorrents_OneFailsOneSucceeds(t *testing.T) {
	dir := t.TempDir()
	mediaRoot := filepath.Join(dir, "media")
	downloadDir := filepath.Join(dir, "downloads")
	require.NoError(t, os.MkdirAll(downloadDir, 0755))

	// t1: valid episode.
	src1 := filepath.Join(downloadDir, "Breaking.Bad.S01E01.mkv")
	require.NoError(t, os.WriteFile(src1, []byte("ep1"), 0644))

	// t2: no episode marker — will fail.
	src2 := filepath.Join(downloadDir, "Breaking.Bad.SpecialFeatures.mkv")
	require.NoError(t, os.WriteFile(src2, []byte("bonus"), 0644))

	meta := &stubMetadata{tv: &domain.TVMatch{ID: 1396, Name: "Breaking Bad", Year: 2008}}
	del := &e2eDeluge{}
	px := newE2EPlex()
	h := newErrE2EHandler(t, meta, del, px, mediaRoot)

	w := httptest.NewRecorder()
	r := postForm(map[string][]string{
		"db": {"TheMovieDB::TV"}, "action": {"move"}, "conflict": {"skip"},
		"output":       {mediaRoot},
		"torrent_ids":  {"tA", "tB"},
		"source_paths": {src1, src2},
	})
	r = r.WithContext(handler.WithUser(r.Context(), &domain.User{PlexToken: "tok"}))
	h.Execute(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	// Partial failure → error toast.
	assert.Contains(t, w.Header().Get("HX-Trigger"), "error")
	// t1 succeeded: moved, deleted.
	assert.FileExists(t, filepath.Join(mediaRoot, "TV", "Breaking Bad", "Season 1", "Breaking Bad - S01E01.mkv"))
	assert.NoFileExists(t, src1)
	assert.Contains(t, del.deletedIDs, "tA")
	// t2 failed: source intact, not deleted.
	assert.FileExists(t, src2)
	assert.NotContains(t, del.deletedIDs, "tB")
	// Plex refreshed because at least one move succeeded.
	px.waitCalled()
	assert.True(t, px.called)

	body := w.Body.String()
	assert.Contains(t, body, "tA|true|true|false|moved and deleted;")
	assert.Contains(t, body, "tB|false|false|true|")
}

// ── nested season folders: one season conflicts, one succeeds ──────────────

func TestE2EError_Anime_NestedFolders_OneSeasonConflictFail(t *testing.T) {
	dir := t.TempDir()
	mediaRoot := filepath.Join(dir, "media")
	torrentRoot := filepath.Join(dir, "downloads", "Sorcerous Stabber Orphen")
	require.NoError(t, os.MkdirAll(filepath.Join(torrentRoot, "S1"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(torrentRoot, "S2"), 0755))

	require.NoError(t, os.WriteFile(
		filepath.Join(torrentRoot, "S1", "Lo stregone Orphen S01E01 [1080p].mkv"),
		[]byte("s1ep1"), 0644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(torrentRoot, "S2", "Lo stregone Orphen S02E01 [1080p].mkv"),
		[]byte("s2ep1"), 0644,
	))

	// Pre-create the S2 target so conflict=fail triggers for that file only.
	s2Target := filepath.Join(mediaRoot, "Anime", "Sorcerous Stabber Orphen", "Season 2", "Sorcerous Stabber Orphen - S02E01.mkv")
	require.NoError(t, os.MkdirAll(filepath.Dir(s2Target), 0755))
	require.NoError(t, os.WriteFile(s2Target, []byte("existing-s2"), 0644))

	meta := &stubMetadata{anime: &domain.AnimeMatch{ID: 1003, Title: "Sorcerous Stabber Orphen", Year: 1998}}
	del := &e2eDeluge{}
	px := newE2EPlex()
	h := newErrE2EHandler(t, meta, del, px, mediaRoot)

	w := httptest.NewRecorder()
	r := postForm(map[string][]string{
		"db": {"AniDB"}, "action": {"move"}, "conflict": {"fail"},
		"output": {mediaRoot}, "torrent_ids": {"t1"}, "source_paths": {torrentRoot},
	})
	r = r.WithContext(handler.WithUser(r.Context(), &domain.User{PlexToken: "tok"}))
	h.Execute(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	// At least one file failed → error toast.
	assert.Contains(t, w.Header().Get("HX-Trigger"), "error")
	// S1 file moved.
	assert.FileExists(t, filepath.Join(mediaRoot, "Anime", "Sorcerous Stabber Orphen", "Season 1", "Sorcerous Stabber Orphen - S01E01.mkv"))
	// S2 target untouched (conflict=fail blocked the write).
	assert.Equal(t, "existing-s2", func() string { b, _ := os.ReadFile(s2Target); return string(b) }())
	// S2 source file still on disk.
	assert.FileExists(t, filepath.Join(torrentRoot, "S2", "Lo stregone Orphen S02E01 [1080p].mkv"))
	// No delete — the engine returned errors so the handler skips cleanup.
	assert.Empty(t, del.deletedIDs)
	assert.False(t, px.called)

	// Outcome: torrent marked failed (not moved+deleted).
	assert.Contains(t, w.Body.String(), fmt.Sprintf("t1|false|false|true|"))
}
