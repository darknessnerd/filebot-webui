package filebot

import (
	"context"
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
