package filebot

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/darknessnerd/filebot-webui/internal/domain"
	"github.com/darknessnerd/filebot-webui/internal/logger"
)

type stubResolver struct {
	movie *domain.MovieMatch
	tv    *domain.TVMatch
	anime *domain.AnimeMatch
	err   error
}

type aidOnlyResolver struct {
	anime      *domain.AnimeMatch
	aidCalls   int
	titleCalls int
}

func (r *aidOnlyResolver) SearchMovie(_ context.Context, _ string, _ int) (*domain.MovieMatch, error) {
	return nil, nil
}

func (r *aidOnlyResolver) SearchTV(_ context.Context, _ string, _ int) (*domain.TVMatch, error) {
	return nil, nil
}

func (r *aidOnlyResolver) SearchAnimeByAID(_ context.Context, _ int) (*domain.AnimeMatch, error) {
	r.aidCalls++
	return r.anime, nil
}

func (r *aidOnlyResolver) SearchAnime(_ context.Context, _ string, _ int) (*domain.AnimeMatch, error) {
	r.titleCalls++
	return nil, errors.New("title lookup should not be called")
}

func (s *stubResolver) SearchMovie(_ context.Context, _ string, _ int) (*domain.MovieMatch, error) {
	return s.movie, s.err
}

func (s *stubResolver) SearchTV(_ context.Context, _ string, _ int) (*domain.TVMatch, error) {
	return s.tv, s.err
}

func (s *stubResolver) SearchAnimeByAID(_ context.Context, _ int) (*domain.AnimeMatch, error) {
	return s.anime, s.err
}

func (s *stubResolver) SearchAnime(_ context.Context, _ string, _ int) (*domain.AnimeMatch, error) {
	return s.anime, s.err
}

func TestInternalEngine_MovieMove(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "Dune.Part.Two.2024.mkv")
	require.NoError(t, os.WriteFile(source, []byte("movie"), 0644))

	engine := NewInternalEngine(&stubResolver{
		movie: &domain.MovieMatch{ID: 1, Title: "Dune Part Two", Year: 2024},
	}, logger.New("error", false))

	result, err := engine.Execute(context.Background(), domain.FileBotJob{
		SourcePaths: []string{source},
		DB:          "TheMovieDB",
		Action:      "move",
		Conflict:    "skip",
		Output:      filepath.Join(dir, "media"),
	})
	require.NoError(t, err)
	assert.NotEmpty(t, result.Successes)
	assert.FileExists(t, filepath.Join(dir, "media", "Movies", "Dune Part Two (2024)", "Dune Part Two.mkv"))
}

func TestInternalEngine_MovieMove_WithSubtitle(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "Dune.Part.Two.2024.mkv")
	sub := filepath.Join(dir, "Dune.Part.Two.2024.en.srt")
	require.NoError(t, os.WriteFile(source, []byte("movie"), 0644))
	require.NoError(t, os.WriteFile(sub, []byte("sub"), 0644))

	engine := NewInternalEngine(&stubResolver{
		movie: &domain.MovieMatch{ID: 1, Title: "Dune Part Two", Year: 2024},
	}, logger.New("error", false))

	result, err := engine.Execute(context.Background(), domain.FileBotJob{
		SourcePaths: []string{source},
		DB:          "TheMovieDB",
		Action:      "move",
		Conflict:    "skip",
		Output:      filepath.Join(dir, "media"),
	})
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(dir, "media", "Movies", "Dune Part Two (2024)", "Dune Part Two.mkv"))
	assert.FileExists(t, filepath.Join(dir, "media", "Movies", "Dune Part Two (2024)", "Dune Part Two.en.srt"))
	assert.Len(t, result.Successes, 2) // video + subtitle
}

func TestInternalEngine_Movie_PerFileTMDB(t *testing.T) {
	dir := t.TempDir()
	calls := 0
	resolver := &countingResolver{
		movie:  &domain.MovieMatch{ID: 1, Title: "Dune Part Two", Year: 2024},
		onCall: func() { calls++ },
	}

	// Both files come from same torrent — user sets job.Query to force one lookup.
	f1 := filepath.Join(dir, "Dune.Part.Two.2024.mkv")
	f2 := filepath.Join(dir, "Dune.Part.Two.2024.extras.mkv")
	require.NoError(t, os.WriteFile(f1, []byte("a"), 0644))
	require.NoError(t, os.WriteFile(f2, []byte("b"), 0644))

	engine := NewInternalEngine(resolver, logger.New("error", false))
	_, err := engine.Execute(context.Background(), domain.FileBotJob{
		SourcePaths: []string{f1, f2},
		DB:          "TheMovieDB",
		Action:      "move",
		Conflict:    "index",
		Query:       "Dune Part Two",
		Output:      filepath.Join(dir, "media"),
	})
	require.NoError(t, err)
	assert.Equal(t, 1, calls, "same query should hit TMDB only once (cached)")
}

func TestInternalEngine_TVTest(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "Breaking.Bad.S01E01.1080p.mkv")
	require.NoError(t, os.WriteFile(source, []byte("episode"), 0644))

	engine := NewInternalEngine(&stubResolver{
		tv: &domain.TVMatch{ID: 1396, Name: "Breaking Bad", Year: 2008},
	}, logger.New("error", false))

	result, err := engine.Execute(context.Background(), domain.FileBotJob{
		SourcePaths: []string{source},
		DB:          "TheMovieDB::TV",
		Action:      "test",
		Conflict:    "skip",
		Output:      filepath.Join(dir, "media"),
	})
	require.NoError(t, err)
	assert.Contains(t, result.Successes[0], "Breaking Bad - S01E01.mkv")
	assert.NoFileExists(t, filepath.Join(dir, "media", "TV"))
}

func TestInternalEngine_TV_MultiEpisode(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		file   string
		wantEp string
	}{
		{"Show.S01E01E02.mkv", "S01E01-E02"},
		{"Show.S01E01-E02.mkv", "S01E01-E02"},
		{"Show.S02E05.mkv", "S02E05"},
	}

	for _, tc := range cases {
		source := filepath.Join(dir, tc.file)
		require.NoError(t, os.WriteFile(source, []byte("ep"), 0644))

		engine := NewInternalEngine(&stubResolver{
			tv: &domain.TVMatch{ID: 1, Name: "Show", Year: 2020},
		}, logger.New("error", false))

		result, err := engine.Execute(context.Background(), domain.FileBotJob{
			SourcePaths: []string{source},
			DB:          "TheMovieDB::TV",
			Action:      "test",
			Conflict:    "skip",
			Output:      filepath.Join(dir, "media"),
		})
		require.NoError(t, err, "file=%s", tc.file)
		assert.Contains(t, result.Successes[0], tc.wantEp, "file=%s", tc.file)
	}
}

func TestInternalEngine_TV_CorpusEpisodePatterns(t *testing.T) {
	engine := NewInternalEngine(&stubResolver{
		tv: &domain.TVMatch{ID: 1, Name: "Corpus Show", Year: 2020},
	}, logger.New("error", false))

	cases := []struct {
		name    string
		source  string
		wantEp  string
	}{
		{"standard SxxExx",         "Corpus.Show.S02E05.1080p.mkv",                    "S02E05"},
		{"SxxExxExx double",        "Corpus.Show.S01E01E02.mkv",                        "S01E01-E02"},
		{"SxxExx-Exx range",        "Corpus.Show.S01E01-E02.mkv",                       "S01E01-E02"},
		{"NxNN format",             "Corpus.Show.2x05.HDTV.mkv",                        "S02E05"},
		{"S01.E01 dot sep",         "Corpus.Show.S01.E01.mkv",                          "S01E01"},
		{"[S01E01] brackets",       "[Group] Corpus Show [S01E01] [1080p].mkv",         "S01E01"},
		{"season_episode underscore","corpus_show_s03e07_1080p.mkv",                    "S03E07"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			result, err := engine.Execute(context.Background(), domain.FileBotJob{
				SourcePaths: []string{tc.source},
				DB:          "TheMovieDB::TV",
				Action:      "test",
				Conflict:    "skip",
				Query:       "Corpus Show",
				Output:      "/media",
			})
			require.NoError(t, err, "source=%s", tc.source)
			assert.Contains(t, result.Successes[0], tc.wantEp, "source=%s", tc.source)
		})
	}
}

func TestInternalEngine_Movie_CorpusQueryNormalization(t *testing.T) {
	engine := NewInternalEngine(&stubResolver{
		movie: &domain.MovieMatch{ID: 1, Title: "Corpus Movie", Year: 2024},
	}, logger.New("error", false))

	cases := []struct {
		name   string
		source string
	}{
		{"dot separated year",        "Corpus.Movie.2024.1080p.BluRay.mkv"},
		{"space separated year",      "Corpus Movie 2024 1080p.mkv"},
		{"year in parens",            "Corpus Movie (2024) BluRay.mkv"},
		{"year in brackets",          "Corpus Movie [2024] 1080p.mkv"},
		{"underscore separated",      "Corpus_Movie_2024_1080p.mkv"},
		{"codec noise",               "Corpus.Movie.2024.2160p.UHD.BluRay.x265.DTS-HD.mkv"},
		{"release group tag",         "Corpus.Movie.2024.BluRay.REMUX.mkv"},
		{"hdr noise",                 "Corpus.Movie.2024.HDR10.DOVI.mkv"},
		{"remastered suffix",         "Corpus Movie (2024) Remastered 1080p.mkv"},
		{"year range in parens",      "Corpus Movie (2024-2025) BluRay.mkv"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			result, err := engine.Execute(context.Background(), domain.FileBotJob{
				SourcePaths: []string{tc.source},
				DB:          "TheMovieDB",
				Action:      "test",
				Conflict:    "skip",
				Output:      "/media",
			})
			require.NoError(t, err, "source=%s", tc.source)
			assert.Contains(t, result.Successes[0], "Corpus Movie", "source=%s", tc.source)
		})
	}
}

func TestInternalEngine_AnimeTest_StagioneMarker(t *testing.T) {
	dir := t.TempDir()
	sourceDir := filepath.Join(dir, "Mushoku Tensei - Stagione 3 (2026) [03-14]")
	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	source := filepath.Join(sourceDir, "episode-03.mkv")
	require.NoError(t, os.WriteFile(source, []byte("episode"), 0644))

	engine := NewInternalEngine(&stubResolver{
		anime: &domain.AnimeMatch{ID: 1, Title: "Mushoku Tensei: Jobless Reincarnation", Year: 2021},
	}, logger.New("error", false))

	result, err := engine.Execute(context.Background(), domain.FileBotJob{
		SourcePaths: []string{source},
		DB:          "AniDB",
		Action:      "test",
		Conflict:    "skip",
		Query:       "aid:1",
		Output:      filepath.Join(dir, "media"),
	})
	require.NoError(t, err)
	assert.Contains(t, result.Successes[0], "Season 3")
	assert.Contains(t, result.Successes[0], "S03E03")
}

func TestInternalEngine_AnimeTest_EpisodeRange(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "Dirty Pair Flash (1994) e01-16.mkv")
	require.NoError(t, os.WriteFile(source, []byte("episode"), 0644))

	engine := NewInternalEngine(&stubResolver{
		anime: &domain.AnimeMatch{ID: 2, Title: "Dirty Pair Flash", Year: 1994},
	}, logger.New("error", false))

	result, err := engine.Execute(context.Background(), domain.FileBotJob{
		SourcePaths: []string{source},
		DB:          "AniDB",
		Action:      "test",
		Conflict:    "skip",
		Query:       "aid:2",
		Output:      filepath.Join(dir, "media"),
	})
	require.NoError(t, err)
	assert.Contains(t, result.Successes[0], "S01E01-E16")
}

func TestInternalEngine_Anime_TitleLookupFallback(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "One Piece S01E01.mkv")
	require.NoError(t, os.WriteFile(source, []byte("episode"), 0644))

	engine := NewInternalEngine(&stubResolver{
		anime: &domain.AnimeMatch{ID: 21, Title: "One Piece", Year: 1999},
	}, logger.New("error", false))

	result, err := engine.Execute(context.Background(), domain.FileBotJob{
		SourcePaths: []string{source},
		DB:          "AniDB",
		Action:      "test",
		Conflict:    "skip",
		Output:      filepath.Join(dir, "media"),
	})
	require.NoError(t, err)
	assert.Contains(t, result.Successes[0], "One Piece - S01E01")
}


func TestInternalEngine_Anime_CorpusEpisodePatterns(t *testing.T) {
	engine := NewInternalEngine(&stubResolver{
		anime: &domain.AnimeMatch{ID: 999, Title: "Corpus Anime", Year: 2026},
	}, logger.New("error", false))

	cases := []struct {
		name    string
		source  string
		wantStr string
	}{
		{
			name:    "progress fraction ongoing",
			source:  "Mushoku Tensei Jobless Reincarnation - Stagione 3 (2026) [03/14] 1080p x264.mkv",
			wantStr: "Season 3/Corpus Anime - S03E03.mkv",
		},
		{
			name:    "progress range fraction",
			source:  "One Piece - Stagione 1 (1999) [01-30/61] 1080p H264.mkv",
			wantStr: "Season 1/Corpus Anime - S01E01-E30.mkv",
		},
		{
			name:    "unknown total episodes",
			source:  "Thunder 3 - Stagione 1 (2026) [04/XX] 1080p x264.mkv",
			wantStr: "Season 1/Corpus Anime - S01E04.mkv",
		},
		{
			name:    "ep range plus ova suffix",
			source:  "Fullmetal Alchemist Brotherhood (2009) [EP 1-64 + 4 OVA] 1080p H265.mkv",
			wantStr: "Season 1/Corpus Anime - S01E01-E64.mkv",
		},
		{
			name:    "bare ep after dash subsplease style",
			source:  "[SubsPlease] Cowboy Bebop - 01 (1080p) [ABCD1234].mkv",
			wantStr: "Season 1/Corpus Anime - S01E01.mkv",
		},
		{
			name:    "bare ep range after dash",
			source:  "[SubsPlease] Cowboy Bebop - 01-26 (1080p) [ABCD1234].mkv",
			wantStr: "Season 1/Corpus Anime - S01E01-E26.mkv",
		},
		{
			name:    "hash episode",
			source:  "Neon Genesis Evangelion #01 (1080p).mkv",
			wantStr: "Season 1/Corpus Anime - S01E01.mkv",
		},
		{
			name:    "ova episode number",
			source:  "Hellsing OVA 3 (2006) [1080p].mkv",
			wantStr: "Season 1/Corpus Anime - S01E03.mkv",
		},
		{
			name:    "special episode number",
			source:  "Sword Art Online SP2 1080p.mkv",
			wantStr: "Season 1/Corpus Anime - S01E02.mkv",
		},
		{
			name:    "part roman numeral",
			source:  "Tenchi Muyo Part III (1992).mkv",
			wantStr: "Season 1/Corpus Anime - S01E03.mkv",
		},
		{
			name:    "part arabic numeral",
			source:  "Macross Part 2 (1984).mkv",
			wantStr: "Season 1/Corpus Anime - S01E02.mkv",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			result, err := engine.Execute(context.Background(), domain.FileBotJob{
				SourcePaths: []string{tc.source},
				DB:          "AniDB",
				Action:      "test",
				Conflict:    "skip",
				Query:       "aid:999",
				Output:      "/media",
			})
			require.NoError(t, err)
			require.NotEmpty(t, result.Successes)
			assert.Contains(t, result.Successes[0], tc.wantStr)
		})
	}
}

func TestInternalEngine_RejectsUnsupportedFilter(t *testing.T) {
	engine := NewInternalEngine(&stubResolver{}, logger.New("error", false))
	_, err := engine.Execute(context.Background(), domain.FileBotJob{
		SourcePaths: []string{"/tmp/file.mkv"},
		DB:          "TheMovieDB",
		Action:      "test",
		Conflict:    "skip",
		Filter:      "age > 7",
		Output:      "/tmp/out",
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrInvalidArg)
}

// --- regression: anime season folder where episode files normalize to empty query ---
// "Lo stregone Orphen (1998-2000) (1080p)" contains episodes like "[Group] - 01 [720p].mkv"
// which strip to "". Engine must fall back to the torrent root path for query derivation.
func TestInternalEngine_Anime_EpisodeFileEmptyQuery_FallsBackToFolder(t *testing.T) {
	dir := t.TempDir()
	seasonDir := filepath.Join(dir, "Lo stregone Orphen (1998-2000) (1080p)")
	require.NoError(t, os.MkdirAll(seasonDir, 0755))
	// episode filename that normalizes to empty: only group tag + episode number
	require.NoError(t, os.WriteFile(
		filepath.Join(seasonDir, "[AnimeGroup] - 01 [720p].mkv"),
		[]byte("data"), 0644,
	))

	gotQuery := ""
	srv := &captureResolver{
		anime: &domain.AnimeMatch{ID: 1003, Title: "Sorcerous Stabber Orphen", Year: 1998},
		onSearchTV: func(q string, _ int) {},
	}
	// capture via SearchAnime
	captured := &captureAnimeResolver{
		inner:   srv,
		onQuery: func(q string) { gotQuery = q },
	}

	engine := NewInternalEngine(captured, logger.New("error", false))
	result, err := engine.Execute(context.Background(), domain.FileBotJob{
		SourcePaths: []string{seasonDir},
		DB:          "AniDB",
		Action:      "test",
		Conflict:    "skip",
		Output:      "/media",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, gotQuery, "query must not be empty — must use folder name as fallback")
	assert.Contains(t, gotQuery, "Orphen")
	assert.NotEmpty(t, result.Successes)
}

type captureAnimeResolver struct {
	inner   metadataResolver
	onQuery func(string)
}

func (r *captureAnimeResolver) SearchMovie(ctx context.Context, q string, y int) (*domain.MovieMatch, error) {
	return r.inner.SearchMovie(ctx, q, y)
}
func (r *captureAnimeResolver) SearchTV(ctx context.Context, q string, y int) (*domain.TVMatch, error) {
	return r.inner.SearchTV(ctx, q, y)
}
func (r *captureAnimeResolver) SearchAnimeByAID(ctx context.Context, aid int) (*domain.AnimeMatch, error) {
	return r.inner.SearchAnimeByAID(ctx, aid)
}
func (r *captureAnimeResolver) SearchAnime(ctx context.Context, q string, y int) (*domain.AnimeMatch, error) {
	if r.onQuery != nil {
		r.onQuery(q)
	}
	return r.inner.SearchAnime(ctx, q, y)
}

// --- regression: nested S1/S2 season subfolders with group-tagged episode filenames ---
// Structure: TorrentRoot/S1/[Group] - 01.mkv, TorrentRoot/S2/[Group] - 01.mkv
// Without recursive=true the walk finds nothing. Auto-recurse must kick in.
// Query must use TorrentRoot name, not the empty-normalizing episode filename.
func TestInternalEngine_Anime_NestedSeasonFolders_AutoRecurse(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "Lo stregone Orphen (1998-2000) (1080p)")
	require.NoError(t, os.MkdirAll(filepath.Join(root, "S1"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "S2"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "S1", "[AnimeGroup] Lo stregone Orphen - 01 [1080p].mkv"), []byte("ep"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "S2", "[AnimeGroup] Lo stregone Orphen - 01 [1080p].mkv"), []byte("ep"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "Lo stregone Orphen SP1 [1080p].mkv"), []byte("sp"), 0644))

	gotQuery := ""
	captured := &captureAnimeResolver{
		inner: &captureResolver{
			anime: &domain.AnimeMatch{ID: 1003, Title: "Sorcerous Stabber Orphen", Year: 1998},
		},
		onQuery: func(q string) { gotQuery = q },
	}

	engine := NewInternalEngine(captured, logger.New("error", false))
	result, err := engine.Execute(context.Background(), domain.FileBotJob{
		SourcePaths: []string{root},
		DB:          "AniDB",
		Action:      "move",
		Conflict:    "skip",
		Recursive:   false, // user did NOT tick recursive — auto-recurse must handle it
		Output:      filepath.Join(dir, "media"),
	})
	require.NoError(t, err)
	assert.Contains(t, gotQuery, "Orphen", "query must come from torrent root, not episode file")
	assert.Len(t, result.Successes, 3, "S1/ep + S2/ep + Speciale must all be processed")
}

// --- regression: season folder torrent — dry-run must walk real directory ---
// Previously collectVideoFiles with action=test appended ".mkv" to the folder name,
// producing "Rick and Morty - Stagione 09 (2026).mkv" with no episode marker.
// Fix: when path exists on disk, walk it even in dry-run mode.

func TestInternalEngine_TV_SeasonFolder_DryRun(t *testing.T) {
	dir := t.TempDir()
	seasonDir := filepath.Join(dir, "Rick and Morty - Stagione 09 (2026)")
	require.NoError(t, os.MkdirAll(seasonDir, 0755))
	episodes := []string{
		"Rick and Morty 09x01 - Tutti pazzi per Morty.mkv",
		"Rick and Morty 09x02 - Sei Giornirick sette notti.mkv",
	}
	for _, ep := range episodes {
		require.NoError(t, os.WriteFile(filepath.Join(seasonDir, ep), []byte("data"), 0644))
	}

	engine := NewInternalEngine(&stubResolver{
		tv: &domain.TVMatch{ID: 60625, Name: "Rick and Morty", Year: 2013},
	}, logger.New("error", false))

	result, err := engine.Execute(context.Background(), domain.FileBotJob{
		SourcePaths: []string{seasonDir},
		DB:          "TheMovieDB::TV",
		Action:      "test",
		Conflict:    "skip",
		Output:      filepath.Join(dir, "media"),
	})
	require.NoError(t, err)
	assert.Len(t, result.Successes, 2, "both episode files must be discovered")
	assert.Contains(t, result.Successes[0], "S09E01")
	assert.Contains(t, result.Successes[1], "S09E02")
	assert.NoDirExists(t, filepath.Join(dir, "media"), "test action must not create dirs")
}

func TestInternalEngine_TV_SeasonFolder_Move(t *testing.T) {
	dir := t.TempDir()
	seasonDir := filepath.Join(dir, "Rick and Morty - Stagione 09 (2026)")
	require.NoError(t, os.MkdirAll(seasonDir, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(seasonDir, "Rick and Morty 09x01 - Tutti pazzi per Morty.mkv"),
		[]byte("data"), 0644,
	))

	engine := NewInternalEngine(&stubResolver{
		tv: &domain.TVMatch{ID: 60625, Name: "Rick and Morty", Year: 2013},
	}, logger.New("error", false))

	_, err := engine.Execute(context.Background(), domain.FileBotJob{
		SourcePaths: []string{seasonDir},
		DB:          "TheMovieDB::TV",
		Action:      "move",
		Conflict:    "skip",
		Output:      filepath.Join(dir, "media"),
	})
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(dir, "media", "TV", "Rick and Morty", "Season 9", "Rick and Morty - S09E01.mkv"))
}

// Virtual path (dev-mode / no disk): path does not exist → synthesize with .mkv suffix.
func TestInternalEngine_TV_VirtualPath_DryRun(t *testing.T) {
	engine := NewInternalEngine(&stubResolver{
		tv: &domain.TVMatch{ID: 1396, Name: "Breaking Bad", Year: 2008},
	}, logger.New("error", false))

	result, err := engine.Execute(context.Background(), domain.FileBotJob{
		SourcePaths: []string{"Breaking.Bad.S01E01.1080p"},
		DB:          "TheMovieDB::TV",
		Action:      "test",
		Conflict:    "skip",
		Output:      "/media",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, result.Successes)
	assert.Contains(t, result.Successes[0], "S01E01")
}

// --- regression: TV filenames with episode titles and streaming source codes ---
// "Futurama.S14E02.Catfish.Hunter.1080p.DSNP.WEB-DL.ENG.ITA.DDP5.1.H264-TheBlackKing.mkv"
// Previously produced query "Futurama Catfish Hunter DSNP DDP5 1 TheBlackKing" → no results.
func TestInternalEngine_TV_EpisodeTitleStripped(t *testing.T) {
	gotQuery := ""
	srv := &captureResolver{
		tv: &domain.TVMatch{ID: 1408, Name: "Futurama", Year: 1999},
		onSearchTV: func(q string, _ int) { gotQuery = q },
	}
	engine := NewInternalEngine(srv, logger.New("error", false))

	result, err := engine.Execute(context.Background(), domain.FileBotJob{
		SourcePaths: []string{"Futurama.S14E02.Catfish.Hunter.1080p.DSNP.WEB-DL.ENG.ITA.DDP5.1.H264-TheBlackKing.mkv"},
		DB:          "TheMovieDB::TV",
		Action:      "test",
		Conflict:    "skip",
		Output:      "/media",
	})
	require.NoError(t, err)
	assert.Equal(t, "Futurama", gotQuery, "episode title and noise must be stripped from query")
	assert.Contains(t, result.Successes[0], "Futurama - S14E02")
}

// "Rick and Morty - Stagione 09 (2026)" is a season folder; individual episode files
// inside carry the SxxExx code. Verify query is clean and year 2026 is extracted
// (SearchTV will retry without year in the TMDB client if needed).
func TestInternalEngine_TV_RickAndMorty_SeasonYear(t *testing.T) {
	gotQuery := ""
	gotYear := -1
	srv := &captureResolver{
		tv: &domain.TVMatch{ID: 60625, Name: "Rick and Morty", Year: 2013},
		onSearchTV: func(q string, y int) { gotQuery = q; gotYear = y },
	}
	engine := NewInternalEngine(srv, logger.New("error", false))

	result, err := engine.Execute(context.Background(), domain.FileBotJob{
		// Individual episode file inside the season folder — as it would appear in Deluge.
		SourcePaths: []string{"Rick and Morty - Stagione 09 (2026) S09E01.mkv"},
		DB:          "TheMovieDB::TV",
		Action:      "test",
		Conflict:    "skip",
		Output:      "/media",
	})
	require.NoError(t, err)
	assert.Equal(t, "Rick and Morty", gotQuery, "episode title and noise stripped")
	assert.Equal(t, 2026, gotYear, "season year passed to SearchTV for retry logic")
	assert.Contains(t, result.Successes[0], "Rick and Morty")
}

// captureResolver wraps stubResolver and records the query passed to SearchTV/SearchMovie.
type captureResolver struct {
	movie      *domain.MovieMatch
	tv         *domain.TVMatch
	anime      *domain.AnimeMatch
	err        error
	onSearchTV func(query string, year int)
}

func (r *captureResolver) SearchMovie(_ context.Context, q string, y int) (*domain.MovieMatch, error) {
	return r.movie, r.err
}
func (r *captureResolver) SearchTV(_ context.Context, q string, y int) (*domain.TVMatch, error) {
	if r.onSearchTV != nil {
		r.onSearchTV(q, y)
	}
	return r.tv, r.err
}
func (r *captureResolver) SearchAnimeByAID(_ context.Context, _ int) (*domain.AnimeMatch, error) {
	return r.anime, r.err
}
func (r *captureResolver) SearchAnime(_ context.Context, _ string, _ int) (*domain.AnimeMatch, error) {
	return r.anime, r.err
}

// --- regression: cross-device copy must preserve file permissions ---
// Bug: os.CreateTemp creates files with 0600; Plex (and other processes) could not
// read moved files because the target ended up root:root 0600 instead of the
// original muadib:muadib 0644.

func TestCopyFile_PreservesMode_0644(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "source.mkv")
	dst := filepath.Join(dir, "dest.mkv")
	require.NoError(t, os.WriteFile(src, []byte("data"), 0644))

	require.NoError(t, copyFile(src, dst))

	info, err := os.Stat(dst)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0644), info.Mode().Perm())
}

func TestCopyFile_PreservesMode_0640(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "source.mkv")
	dst := filepath.Join(dir, "dest.mkv")
	require.NoError(t, os.WriteFile(src, []byte("data"), 0640))

	require.NoError(t, copyFile(src, dst))

	info, err := os.Stat(dst)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0640), info.Mode().Perm())
}

func TestApplyAction_Copy_PreservesMode(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "Movie.mkv")
	dst := filepath.Join(dir, "out", "Movie.mkv")
	require.NoError(t, os.WriteFile(src, []byte("data"), 0644))

	_, err := applyAction("copy", "skip", src, dst)
	require.NoError(t, err)

	info, err := os.Stat(dst)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0644), info.Mode().Perm())
}

func TestApplyAction_MkdirAll_Perms(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "Movie.mkv")
	// deeply nested target to exercise MkdirAll
	dst := filepath.Join(dir, "media", "Movies", "Dune (2021)", "Dune.mkv")
	require.NoError(t, os.WriteFile(src, []byte("data"), 0644))

	_, err := applyAction("move", "skip", src, dst)
	require.NoError(t, err)
	assert.FileExists(t, dst)

	// parent dirs must be traversable (0755)
	info, err := os.Stat(filepath.Join(dir, "media", "Movies"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0755), info.Mode().Perm())
}

// countingResolver lets tests verify TMDB call count.
type countingResolver struct {
	movie  *domain.MovieMatch
	tv     *domain.TVMatch
	anime  *domain.AnimeMatch
	onCall func()
}

func (r *countingResolver) SearchMovie(_ context.Context, _ string, _ int) (*domain.MovieMatch, error) {
	if r.onCall != nil {
		r.onCall()
	}
	return r.movie, nil
}

func (r *countingResolver) SearchTV(_ context.Context, _ string, _ int) (*domain.TVMatch, error) {
	if r.onCall != nil {
		r.onCall()
	}
	return r.tv, nil
}

func (r *countingResolver) SearchAnimeByAID(_ context.Context, _ int) (*domain.AnimeMatch, error) {
	if r.onCall != nil {
		r.onCall()
	}
	return r.anime, nil
}

func (r *countingResolver) SearchAnime(_ context.Context, _ string, _ int) (*domain.AnimeMatch, error) {
	if r.onCall != nil {
		r.onCall()
	}
	return r.anime, nil
}
