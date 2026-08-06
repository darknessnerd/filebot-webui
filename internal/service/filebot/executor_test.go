package filebot

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/darknessnerd/filebot-webui/internal/domain"
	"github.com/darknessnerd/filebot-webui/internal/logger"
)

const mediaRoot = "/media"

type noopEngine struct{}

func (noopEngine) Execute(_ context.Context, _ domain.FileBotJob) (domain.FileBotResult, error) {
	return domain.FileBotResult{}, nil
}

func newExec() *Service {
	return New(mediaRoot, noopEngine{}, logger.New("error", false))
}

func validJob() domain.FileBotJob {
	return domain.FileBotJob{
		SourcePaths: []string{"/downloads/file.mkv"},
		DB:          "TheMovieDB",
		Action:      "move",
		Conflict:    "skip",
		Output:      "/media/movies",
	}
}

// --- allowlist rejection ---

func TestValidate_DB_Rejected(t *testing.T) {
	j := validJob()
	j.DB = "EvilDB; rm -rf /"
	err := newExec().validate(j)
	require.ErrorIs(t, err, domain.ErrInvalidArg)
}

func TestValidate_Action_Rejected(t *testing.T) {
	j := validJob()
	j.Action = "delete"
	err := newExec().validate(j)
	require.ErrorIs(t, err, domain.ErrInvalidArg)
}

func TestValidate_Conflict_Rejected(t *testing.T) {
	j := validJob()
	j.Conflict = "overwrite"
	err := newExec().validate(j)
	require.ErrorIs(t, err, domain.ErrInvalidArg)
}

func TestValidate_AllAllowedDBs(t *testing.T) {
	for _, db := range []string{"TheMovieDB", "TheMovieDB::TV", "AniDB"} {
		j := validJob()
		j.DB = db
		assert.NoError(t, newExec().validate(j), "db=%s", db)
	}
}

func TestValidate_UnsupportedDBs_Rejected(t *testing.T) {
	for _, db := range []string{"TheTVDB", "AcoustID", "OMDb"} {
		j := validJob()
		j.DB = db
		require.ErrorIs(t, newExec().validate(j), domain.ErrInvalidArg, "db=%s", db)
	}
}

func TestValidate_AllAllowedActions(t *testing.T) {
	for _, action := range []string{"move", "copy", "symlink", "hardlink", "test"} {
		j := validJob()
		j.Action = action
		assert.NoError(t, newExec().validate(j), "action=%s", action)
	}
}

// --- path traversal ---

func TestValidate_Output_PathTraversal(t *testing.T) {
	cases := []string{
		"/media/../../etc/passwd",
		"/tmp/evil",
		"../media",
		"/",
	}
	for _, p := range cases {
		j := validJob()
		j.Output = p
		err := newExec().validate(j)
		require.ErrorIs(t, err, domain.ErrInvalidArg, "output=%s", p)
	}
}

func TestValidate_Output_ValidSubpath(t *testing.T) {
	j := validJob()
	j.Output = "/media/movies/2024"
	assert.NoError(t, newExec().validate(j))
}

// --- shell metachar rejection ---


func TestValidate_Filter_Metachar(t *testing.T) {
	j := validJob()
	j.Filter = "age > 7; rm -rf"
	err := newExec().validate(j)
	require.ErrorIs(t, err, domain.ErrInvalidArg)
}

func TestValidate_Query_Metachar(t *testing.T) {
	j := validJob()
	j.Query = "movie & 2024"
	err := newExec().validate(j)
	require.ErrorIs(t, err, domain.ErrInvalidArg)
}

func TestValidate_SourcePaths_Metachar(t *testing.T) {
	chars := []string{
		"/downloads/file$HOME.mkv",
		"/downloads/`id`.mkv",
		"/downloads/a;b.mkv",
		"/downloads/a|b.mkv",
	}
	for _, p := range chars {
		j := validJob()
		j.SourcePaths = []string{p}
		err := newExec().validate(j)
		require.ErrorIs(t, err, domain.ErrInvalidArg, "source_path=%q", p)
	}
}

func TestValidate_SourcePaths_Clean(t *testing.T) {
	j := validJob()
	j.SourcePaths = []string{"/downloads/Movie.Title.2024.mkv", "/downloads/Show S01E01.mkv"}
	assert.NoError(t, newExec().validate(j))
}

// --- output subdir accepted ---

func TestValidate_Output_ExactMediaRoot(t *testing.T) {
	j := validJob()
	j.Output = mediaRoot
	assert.NoError(t, newExec().validate(j))
}

func TestValidate_Output_DeepSubpath(t *testing.T) {
	j := validJob()
	j.Output = filepath.Join(mediaRoot, "Movies", "2024")
	assert.NoError(t, newExec().validate(j))
}
