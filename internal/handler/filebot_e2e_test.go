package handler_test

// End-to-end tests: real InternalEngine + real filesystem, all external network
// calls (Deluge, TMDB, Plex) replaced by in-process stubs.
// Covers the full handler → service → engine → FS → delete → plex-refresh chain.
//
// Scenario matrix:
//
//	Test                                  DB              Torrent shape               Action  Asserts
//	────────────────────────────────────  ──────────────  ──────────────────────────  ──────  ───────────────────────────────────────────────
//	Movie_MoveDeleteRefresh               TheMovieDB      single flat file            move    file at Movies/<title>/; delete; plex refresh
//	Movie_WithSubtitle_MovesBothFiles     TheMovieDB      flat file + companion .srt  move    video + .srt in same target dir
//	Movie_TestAction_NoMutation           TheMovieDB      single flat file            test    no FS change; no delete; no plex
//	TV_MoveDeleteRefresh                  TheMovieDB::TV  single flat file            move    file at TV/<show>/Season N/; delete; plex refresh
//	TV_SingleEpisodeInFolder_Move         TheMovieDB::TV  folder with one episode     move    file moved; folder cleaned up; delete; plex
//	TV_SeasonFolder_MoveDeleteRefresh     TheMovieDB::TV  folder with multiple eps    move    all eps at Season N/; folder cleaned up; delete; plex
//	MultipleTorrents_BothSucceed          TheMovieDB::TV  two flat files (2 torrents) move    both moved; both deleted; one plex refresh
//	Anime_SingleFlatFile_Move             AniDB           single flat file            move    file at Anime/<title>/Season N/; delete; plex
//	Anime_SeasonFolder_MoveDeleteRefresh  AniDB           folder with one episode     move    file moved; folder cleaned up; delete; plex
//	Anime_MultiEpisodeFlatFolder_Move     AniDB           folder with 3 episodes      move    all 3 eps at Season N/; folder cleaned up; delete; plex
//	Anime_NestedSeasonFolders_AutoRecurse AniDB           folder/S1/ + folder/S2/     move    auto-recurse; both seasons moved; folder cleaned up

import (
	"context"
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

// stubMetadata implements filebot.MetadataResolver with fixed responses.
type stubMetadata struct {
	movie *domain.MovieMatch
	tv    *domain.TVMatch
	anime *domain.AnimeMatch
}

func (s *stubMetadata) SearchMovie(_ context.Context, _ string, _ int) (*domain.MovieMatch, error) {
	return s.movie, nil
}
func (s *stubMetadata) SearchTV(_ context.Context, _ string, _ int) (*domain.TVMatch, error) {
	return s.tv, nil
}
func (s *stubMetadata) SearchAnime(_ context.Context, _ string, _ int) (*domain.AnimeMatch, error) {
	return s.anime, nil
}
func (s *stubMetadata) SearchAnimeByAID(_ context.Context, _ int) (*domain.AnimeMatch, error) {
	return s.anime, nil
}

// e2eDeluge is a minimal Deluge stub that records delete calls.
type e2eDeluge struct {
	torrents   []domain.Torrent
	deletedIDs []string
}

func (d *e2eDeluge) ListCompleted(_ context.Context) ([]domain.Torrent, error) {
	return d.torrents, nil
}
func (d *e2eDeluge) DeleteTorrent(_ context.Context, id string) error {
	d.deletedIDs = append(d.deletedIDs, id)
	return nil
}

func (d *e2eDeluge) deletedID() string {
	if len(d.deletedIDs) == 1 {
		return d.deletedIDs[0]
	}
	return ""
}

// e2ePlex records whether RefreshLibraries was called.
// waitCalled() blocks until the goroutine-fired refresh arrives (buffered channel,
// safe to call even when refresh is not expected — just don't call waitCalled then).
type e2ePlex struct {
	called bool
	ch     chan struct{}
}

func newE2EPlex() *e2ePlex { return &e2ePlex{ch: make(chan struct{}, 1)} }

func (p *e2ePlex) RefreshLibraries(_ context.Context, _ string) error {
	p.called = true
	if p.ch != nil {
		p.ch <- struct{}{}
	}
	return nil
}

// waitCalled blocks until RefreshLibraries is invoked.
func (p *e2ePlex) waitCalled() { <-p.ch }

// newE2EHandler wires a real InternalEngine (with stub metadata resolver) to the
// FileBotHandler. mediaRoot and downloadDir are temp directories owned by the test.
func newE2EHandler(
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

func TestE2E_Movie_MoveDeleteRefresh(t *testing.T) {
	dir := t.TempDir()
	mediaRoot := filepath.Join(dir, "media")
	downloadDir := filepath.Join(dir, "downloads")
	require.NoError(t, os.MkdirAll(downloadDir, 0755))

	// Create a real source file in the download dir.
	src := filepath.Join(downloadDir, "Dune.Part.Two.2024.1080p.mkv")
	require.NoError(t, os.WriteFile(src, []byte("movie data"), 0644))

	meta := &stubMetadata{movie: &domain.MovieMatch{ID: 1, Title: "Dune Part Two", Year: 2024}}
	del := &e2eDeluge{torrents: []domain.Torrent{
		{ID: "t1", Name: "Dune.Part.Two.2024.1080p.mkv", DownloadPath: downloadDir},
	}}
	px := newE2EPlex()

	h := newE2EHandler(t, meta, del, px, mediaRoot)

	fields := map[string][]string{
		"db":           {"TheMovieDB"},
		"action":       {"move"},
		"conflict":     {"skip"},
		"output":       {mediaRoot},
		"torrent_ids":  {"t1"},
		"source_paths": {src},
	}
	w := httptest.NewRecorder()
	r := postForm(fields)
	r = r.WithContext(handler.WithUser(r.Context(), &domain.User{PlexToken: "tok"}))
	h.Execute(w, r)

	require.Equal(t, http.StatusOK, w.Code)

	// File moved to correct location.
	assert.FileExists(t, filepath.Join(mediaRoot, "Movies", "Dune Part Two (2024)", "Dune Part Two.mkv"))
	// Source file gone.
	assert.NoFileExists(t, src)
	// downloadDir is the shared Deluge location (not a torrent-owned folder) — must NOT be removed.
	// Deluge entry removed.
	assert.Equal(t, "t1", del.deletedID())
	// Plex refreshed.
	px.waitCalled()
	assert.True(t, px.called)

	body := w.Body.String()
	assert.Contains(t, body, "t1|true|true|false|moved and deleted;")
	assert.Contains(t, w.Header().Get("HX-Trigger"), "success")
}

func TestE2E_TV_MoveDeleteRefresh(t *testing.T) {
	dir := t.TempDir()
	mediaRoot := filepath.Join(dir, "media")
	downloadDir := filepath.Join(dir, "downloads")
	require.NoError(t, os.MkdirAll(downloadDir, 0755))

	src := filepath.Join(downloadDir, "Breaking.Bad.S01E01.1080p.mkv")
	require.NoError(t, os.WriteFile(src, []byte("episode data"), 0644))

	meta := &stubMetadata{tv: &domain.TVMatch{ID: 1396, Name: "Breaking Bad", Year: 2008}}
	del := &e2eDeluge{torrents: []domain.Torrent{
		{ID: "t2", Name: "Breaking.Bad.S01E01.1080p.mkv", DownloadPath: downloadDir},
	}}
	px := newE2EPlex()

	h := newE2EHandler(t, meta, del, px, mediaRoot)

	fields := map[string][]string{
		"db":           {"TheMovieDB::TV"},
		"action":       {"move"},
		"conflict":     {"skip"},
		"output":       {mediaRoot},
		"torrent_ids":  {"t2"},
		"source_paths": {src},
	}
	w := httptest.NewRecorder()
	r := postForm(fields)
	r = r.WithContext(handler.WithUser(r.Context(), &domain.User{PlexToken: "tok"}))
	h.Execute(w, r)

	require.Equal(t, http.StatusOK, w.Code)

	assert.FileExists(t, filepath.Join(mediaRoot, "TV", "Breaking Bad", "Season 1", "Breaking Bad - S01E01.mkv"))
	assert.NoFileExists(t, src)
	assert.Equal(t, "t2", del.deletedID())
	px.waitCalled()
	assert.True(t, px.called)
	assert.Contains(t, w.Body.String(), "t2|true|true|false|moved and deleted;")
}

func TestE2E_Anime_SeasonFolder_MoveDeleteRefresh(t *testing.T) {
	dir := t.TempDir()
	mediaRoot := filepath.Join(dir, "media")
	// Torrent is a folder containing one episode file.
	torrentRoot := filepath.Join(dir, "downloads", "One Piece S01")
	require.NoError(t, os.MkdirAll(torrentRoot, 0755))

	src := filepath.Join(torrentRoot, "One.Piece.S01E01.mkv")
	require.NoError(t, os.WriteFile(src, []byte("anime data"), 0644))

	meta := &stubMetadata{anime: &domain.AnimeMatch{ID: 21, Title: "One Piece", Year: 1999}}
	del := &e2eDeluge{}
	px := newE2EPlex()

	h := newE2EHandler(t, meta, del, px, mediaRoot)

	fields := map[string][]string{
		"db":           {"AniDB"},
		"action":       {"move"},
		"conflict":     {"skip"},
		"output":       {mediaRoot},
		"torrent_ids":  {"t3"},
		"source_paths": {torrentRoot},
	}
	w := httptest.NewRecorder()
	r := postForm(fields)
	r = r.WithContext(handler.WithUser(r.Context(), &domain.User{PlexToken: "tok"}))
	h.Execute(w, r)

	require.Equal(t, http.StatusOK, w.Code)

	assert.FileExists(t, filepath.Join(mediaRoot, "Anime", "One Piece", "Season 1", "One Piece - S01E01.mkv"))
	assert.NoFileExists(t, src)
	// Season folder should be cleaned up after move.
	assert.NoDirExists(t, torrentRoot)
	assert.Equal(t, "t3", del.deletedID())
	px.waitCalled()
	assert.True(t, px.called)
	assert.Contains(t, w.Body.String(), "t3|true|true|false|moved and deleted;")
}

// Movie with a companion subtitle: both video and .srt must land in the same target dir.
func TestE2E_Movie_WithSubtitle_MovesBothFiles(t *testing.T) {
	dir := t.TempDir()
	mediaRoot := filepath.Join(dir, "media")
	downloadDir := filepath.Join(dir, "downloads")
	require.NoError(t, os.MkdirAll(downloadDir, 0755))

	src := filepath.Join(downloadDir, "Dune.Part.Two.2024.mkv")
	sub := filepath.Join(downloadDir, "Dune.Part.Two.2024.en.srt")
	require.NoError(t, os.WriteFile(src, []byte("movie"), 0644))
	require.NoError(t, os.WriteFile(sub, []byte("subtitle"), 0644))

	meta := &stubMetadata{movie: &domain.MovieMatch{ID: 1, Title: "Dune Part Two", Year: 2024}}
	del := &e2eDeluge{}
	px := newE2EPlex()
	h := newE2EHandler(t, meta, del, px, mediaRoot)

	w := httptest.NewRecorder()
	r := postForm(map[string][]string{
		"db": {"TheMovieDB"}, "action": {"move"}, "conflict": {"skip"},
		"output": {mediaRoot}, "torrent_ids": {"t1"}, "source_paths": {src},
	})
	r = r.WithContext(handler.WithUser(r.Context(), &domain.User{PlexToken: "tok"}))
	h.Execute(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	targetDir := filepath.Join(mediaRoot, "Movies", "Dune Part Two (2024)")
	assert.FileExists(t, filepath.Join(targetDir, "Dune Part Two.mkv"))
	assert.FileExists(t, filepath.Join(targetDir, "Dune Part Two.en.srt"))
	assert.NoFileExists(t, src)
	assert.NoFileExists(t, sub)
}

// action=test must produce TEST output lines but must not move any file or call delete/Plex.
func TestE2E_Movie_TestAction_NoMutation(t *testing.T) {
	dir := t.TempDir()
	mediaRoot := filepath.Join(dir, "media")
	downloadDir := filepath.Join(dir, "downloads")
	require.NoError(t, os.MkdirAll(downloadDir, 0755))

	src := filepath.Join(downloadDir, "Dune.Part.Two.2024.mkv")
	require.NoError(t, os.WriteFile(src, []byte("movie"), 0644))

	meta := &stubMetadata{movie: &domain.MovieMatch{ID: 1, Title: "Dune Part Two", Year: 2024}}
	del := &e2eDeluge{}
	px := newE2EPlex()
	h := newE2EHandler(t, meta, del, px, mediaRoot)

	w := httptest.NewRecorder()
	r := postForm(map[string][]string{
		"db": {"TheMovieDB"}, "action": {"test"}, "conflict": {"skip"},
		"output": {mediaRoot}, "torrent_ids": {"t1"}, "source_paths": {src},
	})
	r = r.WithContext(handler.WithUser(r.Context(), &domain.User{PlexToken: "tok"}))
	h.Execute(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	// Source untouched.
	assert.FileExists(t, src)
	// No media dir created.
	assert.NoDirExists(t, mediaRoot)
	// No delete, no Plex.
	assert.Empty(t, del.deletedIDs)
	assert.False(t, px.called)
	// Result body shows TEST line.
	assert.Contains(t, w.Body.String(), "t1|false|false|false|not moved (test action);")
}

// Two torrents submitted together — both succeed — both deleted — one Plex refresh.
func TestE2E_MultipleTorrents_BothSucceed(t *testing.T) {
	dir := t.TempDir()
	mediaRoot := filepath.Join(dir, "media")
	downloadDir := filepath.Join(dir, "downloads")
	require.NoError(t, os.MkdirAll(downloadDir, 0755))

	src1 := filepath.Join(downloadDir, "Breaking.Bad.S01E01.mkv")
	src2 := filepath.Join(downloadDir, "Breaking.Bad.S01E02.mkv")
	require.NoError(t, os.WriteFile(src1, []byte("ep1"), 0644))
	require.NoError(t, os.WriteFile(src2, []byte("ep2"), 0644))

	meta := &stubMetadata{tv: &domain.TVMatch{ID: 1396, Name: "Breaking Bad", Year: 2008}}
	del := &e2eDeluge{}
	px := newE2EPlex()
	h := newE2EHandler(t, meta, del, px, mediaRoot)

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
	seasonDir := filepath.Join(mediaRoot, "TV", "Breaking Bad", "Season 1")
	assert.FileExists(t, filepath.Join(seasonDir, "Breaking Bad - S01E01.mkv"))
	assert.FileExists(t, filepath.Join(seasonDir, "Breaking Bad - S01E02.mkv"))
	assert.NoFileExists(t, src1)
	assert.NoFileExists(t, src2)
	assert.ElementsMatch(t, []string{"tA", "tB"}, del.deletedIDs)
	px.waitCalled()
	assert.True(t, px.called)
	body := w.Body.String()
	assert.Contains(t, body, "tA|true|true|false|moved and deleted;")
	assert.Contains(t, body, "tB|true|true|false|moved and deleted;")
}

// TV season-folder torrent: directory of per-episode files, no recursive flag needed.
func TestE2E_TV_SeasonFolder_MoveDeleteRefresh(t *testing.T) {
	dir := t.TempDir()
	mediaRoot := filepath.Join(dir, "media")
	torrentRoot := filepath.Join(dir, "downloads", "Rick and Morty - Stagione 09 (2026)")
	require.NoError(t, os.MkdirAll(torrentRoot, 0755))

	eps := []string{
		"Rick and Morty 09x01 - Tutti pazzi per Morty.mkv",
		"Rick and Morty 09x02 - Sei Giornirick sette notti.mkv",
	}
	for _, ep := range eps {
		require.NoError(t, os.WriteFile(filepath.Join(torrentRoot, ep), []byte("data"), 0644))
	}

	meta := &stubMetadata{tv: &domain.TVMatch{ID: 60625, Name: "Rick and Morty", Year: 2013}}
	del := &e2eDeluge{}
	px := newE2EPlex()
	h := newE2EHandler(t, meta, del, px, mediaRoot)

	w := httptest.NewRecorder()
	r := postForm(map[string][]string{
		"db": {"TheMovieDB::TV"}, "action": {"move"}, "conflict": {"skip"},
		"output": {mediaRoot}, "torrent_ids": {"t4"}, "source_paths": {torrentRoot},
	})
	r = r.WithContext(handler.WithUser(r.Context(), &domain.User{PlexToken: "tok"}))
	h.Execute(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	seasonDir := filepath.Join(mediaRoot, "TV", "Rick and Morty", "Season 9")
	assert.FileExists(t, filepath.Join(seasonDir, "Rick and Morty - S09E01.mkv"))
	assert.FileExists(t, filepath.Join(seasonDir, "Rick and Morty - S09E02.mkv"))
	assert.NoDirExists(t, torrentRoot)
	assert.Equal(t, "t4", del.deletedID())
	px.waitCalled()
	assert.True(t, px.called)
}

// Anime with nested S1/S2 subfolders: auto-recurse must process all episodes without
// the user ticking the recursive checkbox.
func TestE2E_Anime_NestedSeasonFolders_AutoRecurse(t *testing.T) {
	dir := t.TempDir()
	mediaRoot := filepath.Join(dir, "media")
	torrentRoot := filepath.Join(dir, "downloads", "Lo stregone Orphen (1998-2000)")
	require.NoError(t, os.MkdirAll(filepath.Join(torrentRoot, "S1"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(torrentRoot, "S2"), 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(torrentRoot, "S1", "Lo stregone Orphen S01E01 [1080p].mkv"),
		[]byte("s1e1"), 0644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(torrentRoot, "S2", "Lo stregone Orphen S02E01 [1080p].mkv"),
		[]byte("s2e1"), 0644,
	))

	meta := &stubMetadata{anime: &domain.AnimeMatch{ID: 1003, Title: "Sorcerous Stabber Orphen", Year: 1998}}
	del := &e2eDeluge{}
	px := newE2EPlex()
	h := newE2EHandler(t, meta, del, px, mediaRoot)

	w := httptest.NewRecorder()
	r := postForm(map[string][]string{
		"db": {"AniDB"}, "action": {"move"}, "conflict": {"skip"},
		"output": {mediaRoot}, "torrent_ids": {"t5"}, "source_paths": {torrentRoot},
		// recursive intentionally omitted — auto-recurse must handle it
	})
	r = r.WithContext(handler.WithUser(r.Context(), &domain.User{PlexToken: "tok"}))
	h.Execute(w, r)

	require.Equal(t, http.StatusOK, w.Code)

	animeDir := filepath.Join(mediaRoot, "Anime", "Sorcerous Stabber Orphen")
	assert.FileExists(t, filepath.Join(animeDir, "Season 1", "Sorcerous Stabber Orphen - S01E01.mkv"))
	assert.FileExists(t, filepath.Join(animeDir, "Season 2", "Sorcerous Stabber Orphen - S02E01.mkv"))
	assert.NoDirExists(t, torrentRoot)
	assert.Equal(t, "t5", del.deletedID())
	px.waitCalled()
	assert.True(t, px.called)
	assert.Contains(t, w.Header().Get("HX-Trigger"), "success")
}

// Torrent is a folder containing a single TV episode file (common pattern: the torrent
// root IS the season folder, one .mkv inside).
func TestE2E_TV_SingleEpisodeInFolder_Move(t *testing.T) {
	dir := t.TempDir()
	mediaRoot := filepath.Join(dir, "media")
	torrentRoot := filepath.Join(dir, "downloads", "Breaking.Bad.S01E01.1080p")
	require.NoError(t, os.MkdirAll(torrentRoot, 0755))

	src := filepath.Join(torrentRoot, "Breaking.Bad.S01E01.1080p.mkv")
	require.NoError(t, os.WriteFile(src, []byte("ep"), 0644))

	meta := &stubMetadata{tv: &domain.TVMatch{ID: 1396, Name: "Breaking Bad", Year: 2008}}
	del := &e2eDeluge{}
	px := newE2EPlex()
	h := newE2EHandler(t, meta, del, px, mediaRoot)

	w := httptest.NewRecorder()
	r := postForm(map[string][]string{
		"db": {"TheMovieDB::TV"}, "action": {"move"}, "conflict": {"skip"},
		"output": {mediaRoot}, "torrent_ids": {"t6"}, "source_paths": {torrentRoot},
	})
	r = r.WithContext(handler.WithUser(r.Context(), &domain.User{PlexToken: "tok"}))
	h.Execute(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	assert.FileExists(t, filepath.Join(mediaRoot, "TV", "Breaking Bad", "Season 1", "Breaking Bad - S01E01.mkv"))
	assert.NoFileExists(t, src)
	assert.NoDirExists(t, torrentRoot)
	assert.Equal(t, "t6", del.deletedID())
	px.waitCalled()
	assert.True(t, px.called)
}

// Torrent is a single anime episode flat file (no wrapping folder).
func TestE2E_Anime_SingleFlatFile_Move(t *testing.T) {
	dir := t.TempDir()
	mediaRoot := filepath.Join(dir, "media")
	downloadDir := filepath.Join(dir, "downloads")
	require.NoError(t, os.MkdirAll(downloadDir, 0755))

	src := filepath.Join(downloadDir, "One.Piece.S01E01.mkv")
	require.NoError(t, os.WriteFile(src, []byte("ep"), 0644))

	meta := &stubMetadata{anime: &domain.AnimeMatch{ID: 21, Title: "One Piece", Year: 1999}}
	del := &e2eDeluge{}
	px := newE2EPlex()
	h := newE2EHandler(t, meta, del, px, mediaRoot)

	w := httptest.NewRecorder()
	r := postForm(map[string][]string{
		"db": {"AniDB"}, "action": {"move"}, "conflict": {"skip"},
		"output": {mediaRoot}, "torrent_ids": {"t7"}, "source_paths": {src},
	})
	r = r.WithContext(handler.WithUser(r.Context(), &domain.User{PlexToken: "tok"}))
	h.Execute(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	assert.FileExists(t, filepath.Join(mediaRoot, "Anime", "One Piece", "Season 1", "One Piece - S01E01.mkv"))
	assert.NoFileExists(t, src)
	assert.Equal(t, "t7", del.deletedID())
	px.waitCalled()
	assert.True(t, px.called)
}

// Torrent is a flat folder containing multiple anime episode files (all same season).
func TestE2E_Anime_MultiEpisodeFlatFolder_Move(t *testing.T) {
	dir := t.TempDir()
	mediaRoot := filepath.Join(dir, "media")
	torrentRoot := filepath.Join(dir, "downloads", "One Piece - Season 1")
	require.NoError(t, os.MkdirAll(torrentRoot, 0755))

	for _, name := range []string{
		"One.Piece.S01E01.mkv",
		"One.Piece.S01E02.mkv",
		"One.Piece.S01E03.mkv",
	} {
		require.NoError(t, os.WriteFile(filepath.Join(torrentRoot, name), []byte("ep"), 0644))
	}

	meta := &stubMetadata{anime: &domain.AnimeMatch{ID: 21, Title: "One Piece", Year: 1999}}
	del := &e2eDeluge{}
	px := newE2EPlex()
	h := newE2EHandler(t, meta, del, px, mediaRoot)

	w := httptest.NewRecorder()
	r := postForm(map[string][]string{
		"db": {"AniDB"}, "action": {"move"}, "conflict": {"skip"},
		"output": {mediaRoot}, "torrent_ids": {"t8"}, "source_paths": {torrentRoot},
	})
	r = r.WithContext(handler.WithUser(r.Context(), &domain.User{PlexToken: "tok"}))
	h.Execute(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	seasonDir := filepath.Join(mediaRoot, "Anime", "One Piece", "Season 1")
	assert.FileExists(t, filepath.Join(seasonDir, "One Piece - S01E01.mkv"))
	assert.FileExists(t, filepath.Join(seasonDir, "One Piece - S01E02.mkv"))
	assert.FileExists(t, filepath.Join(seasonDir, "One Piece - S01E03.mkv"))
	assert.NoDirExists(t, torrentRoot)
	assert.Equal(t, "t8", del.deletedID())
	px.waitCalled()
	assert.True(t, px.called)
	assert.Contains(t, w.Header().Get("HX-Trigger"), "success")
}
