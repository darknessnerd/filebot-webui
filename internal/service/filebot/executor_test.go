package filebot

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/darknessnerd/filebot-webui/internal/domain"
	"github.com/darknessnerd/filebot-webui/internal/logger"
)

const mediaRoot = "/media"

func newExec() *Executor {
	return NewExecutor(mediaRoot, "filebot", logger.New("error", false))
}

func validJob() domain.FileBotJob {
	return domain.FileBotJob{
		SourcePaths: []string{"/downloads/file.mkv"},
		DB:          "TheMovieDB",
		Action:      "move",
		Conflict:    "skip",
		LogLevel:    "info",
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

func TestValidate_LogLevel_Rejected(t *testing.T) {
	j := validJob()
	j.LogLevel = "verbose"
	err := newExec().validate(j)
	require.ErrorIs(t, err, domain.ErrInvalidArg)
}

func TestValidate_AllAllowedDBs(t *testing.T) {
	dbs := []string{"TheMovieDB", "TheMovieDB::TV", "TheTVDB", "AniDB", "AcoustID", "OMDb"}
	for _, db := range dbs {
		j := validJob()
		j.DB = db
		assert.NoError(t, newExec().validate(j), "db=%s", db)
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

func TestValidate_Format_Metachar(t *testing.T) {
	chars := []string{"$HOME", "`id`", "a;b", "a&b", "a|b", "a>b", "a<b", "a\nb", "a\rb"}
	for _, v := range chars {
		j := validJob()
		j.Format = v
		err := newExec().validate(j)
		require.ErrorIs(t, err, domain.ErrInvalidArg, "format=%q", v)
	}
}

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

func TestBuildArgs_OptionalFlagsPresent(t *testing.T) {
	j := domain.FileBotJob{
		SourcePaths: []string{"/downloads/file.mkv"},
		DB:          "TheTVDB",
		Action:      "copy",
		Conflict:    "auto",
		LogLevel:    "fine",
		Output:      "/media",
		Format:      "{n}/Season {s}/{n} - {s00e00}",
		Filter:      "age > 0",
		Query:       "Breaking Bad",
		Recursive:   true,
	}
	args := newExec().buildArgs(j)
	joined := strings.Join(args, " ")
	assert.Contains(t, joined, "--format")
	assert.Contains(t, joined, "--filter")
	assert.Contains(t, joined, "--q")
	assert.Contains(t, joined, "-r")
}

func TestBuildArgs_OptionalFlagsAbsent(t *testing.T) {
	j := validJob() // Format/Filter/Query all empty, Recursive false
	args := newExec().buildArgs(j)
	assert.NotContains(t, args, "--format")
	assert.NotContains(t, args, "--filter")
	assert.NotContains(t, args, "--q")
	assert.NotContains(t, args, "-r")
}

// --- happy path with stub binary ---

func TestExecute_HappyPath_StubBinary(t *testing.T) {
	dir := t.TempDir()

	stub := filepath.Join(dir, "filebot")
	err := os.WriteFile(stub, []byte("#!/bin/sh\necho 'renamed ok'\nexit 0\n"), 0755)
	require.NoError(t, err)

	mediaDir := filepath.Join(dir, "media")
	require.NoError(t, os.MkdirAll(mediaDir, 0755))

	e := NewExecutor(mediaDir, stub, logger.New("error", false))
	j := domain.FileBotJob{
		SourcePaths: []string{filepath.Join(dir, "file.mkv")},
		DB:          "TheMovieDB",
		Action:      "test",
		Conflict:    "skip",
		LogLevel:    "info",
		Output:      mediaDir,
	}

	result, err := e.Execute(context.Background(), j)
	require.NoError(t, err)
	assert.Contains(t, result.RawOutput, "renamed ok")
}

// --- exit code 3 regression ---

// TestExecute_Exit3_MoveAction_TreatedAsSuccess guards the bug where FileBot exits 3
// ("No input files") after a successful move and the executor incorrectly
// reported it as a failure, blocking torrent deletion and Plex refresh.
func TestExecute_Exit3_MoveAction_TreatedAsSuccess(t *testing.T) {
	dir := t.TempDir()
	stub := filepath.Join(dir, "filebot")
	require.NoError(t, os.WriteFile(stub, []byte("#!/bin/sh\necho 'No input files'\nexit 3\n"), 0755))

	mediaDir := filepath.Join(dir, "media")
	require.NoError(t, os.MkdirAll(mediaDir, 0755))

	e := NewExecutor(mediaDir, stub, logger.New("error", false))
	j := domain.FileBotJob{
		SourcePaths: []string{filepath.Join(dir, "file.mkv")},
		DB:          "TheMovieDB",
		Action:      "move",
		Conflict:    "skip",
		LogLevel:    "info",
		Output:      mediaDir,
	}

	result, err := e.Execute(context.Background(), j)
	require.NoError(t, err, "exit 3 on move must not return an error")
	assert.Empty(t, result.Errors, "exit 3 on move must not populate result.Errors")
	assert.NotEmpty(t, result.Successes, "exit 3 on move must populate result.Successes")
	assert.Contains(t, result.RawOutput, "No input files", "raw output must be preserved")
}

// TestExecute_Exit3_NonMoveAction_StillError verifies exit 3 is only forgiven for
// action=move; for other actions it indicates a real problem (e.g. bad source path).
func TestExecute_Exit3_NonMoveAction_StillError(t *testing.T) {
	dir := t.TempDir()
	stub := filepath.Join(dir, "filebot")
	require.NoError(t, os.WriteFile(stub, []byte("#!/bin/sh\nexit 3\n"), 0755))

	mediaDir := filepath.Join(dir, "media")
	require.NoError(t, os.MkdirAll(mediaDir, 0755))

	e := NewExecutor(mediaDir, stub, logger.New("error", false))
	j := domain.FileBotJob{
		SourcePaths: []string{filepath.Join(dir, "file.mkv")},
		DB:          "TheMovieDB",
		Action:      "copy",
		Conflict:    "skip",
		LogLevel:    "info",
		Output:      mediaDir,
	}

	_, err := e.Execute(context.Background(), j)
	require.Error(t, err, "exit 3 on non-move action must return an error")
	assert.ErrorIs(t, err, domain.ErrFileBotFailed)
}

func TestExecute_Exit1_StillError(t *testing.T) {
	dir := t.TempDir()
	stub := filepath.Join(dir, "filebot")
	require.NoError(t, os.WriteFile(stub, []byte("#!/bin/sh\necho 'rename failed' >&2\nexit 1\n"), 0755))

	mediaDir := filepath.Join(dir, "media")
	require.NoError(t, os.MkdirAll(mediaDir, 0755))

	e := NewExecutor(mediaDir, stub, logger.New("error", false))
	j := domain.FileBotJob{
		SourcePaths: []string{filepath.Join(dir, "file.mkv")},
		DB:          "TheMovieDB",
		Action:      "move",
		Conflict:    "skip",
		LogLevel:    "info",
		Output:      mediaDir,
	}

	result, err := e.Execute(context.Background(), j)
	require.Error(t, err, "exit 1 must return an error")
	assert.ErrorIs(t, err, domain.ErrFileBotFailed)
	assert.NotEmpty(t, result.Errors)
}

func TestExecute_Exit2_StillError(t *testing.T) {
	dir := t.TempDir()
	stub := filepath.Join(dir, "filebot")
	require.NoError(t, os.WriteFile(stub, []byte("#!/bin/sh\nexit 2\n"), 0755))

	mediaDir := filepath.Join(dir, "media")
	require.NoError(t, os.MkdirAll(mediaDir, 0755))

	e := NewExecutor(mediaDir, stub, logger.New("error", false))
	j := domain.FileBotJob{
		SourcePaths: []string{filepath.Join(dir, "file.mkv")},
		DB:          "TheMovieDB",
		Action:      "move",
		Conflict:    "skip",
		LogLevel:    "info",
		Output:      mediaDir,
	}

	_, err := e.Execute(context.Background(), j)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrFileBotFailed)
}

func TestExecute_ContextCancellation(t *testing.T) {
	dir := t.TempDir()
	stub := filepath.Join(dir, "filebot")
	err := os.WriteFile(stub, []byte("#!/bin/sh\nsleep 10\n"), 0755)
	require.NoError(t, err)

	mediaDir := filepath.Join(dir, "media")
	require.NoError(t, os.MkdirAll(mediaDir, 0755))

	e := NewExecutor(mediaDir, stub, logger.New("error", false))
	j := validJob()
	j.Output = mediaDir

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = e.Execute(ctx, j)
	assert.Error(t, err)
}
