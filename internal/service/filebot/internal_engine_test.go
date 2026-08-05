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
	err   error
}

func (s *stubResolver) SearchMovie(_ context.Context, _ string, _ int) (*domain.MovieMatch, error) {
	return s.movie, s.err
}

func (s *stubResolver) SearchTV(_ context.Context, _ string, _ int) (*domain.TVMatch, error) {
	return s.tv, s.err
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
		movie: &domain.MovieMatch{ID: 1, Title: "Dune Part Two", Year: 2024},
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
