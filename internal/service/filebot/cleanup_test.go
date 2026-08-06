package filebot

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/darknessnerd/filebot-webui/internal/logger"
)

func TestCleanupOrphanedTemps_RemovesTemps(t *testing.T) {
	dir := t.TempDir()
	log := logger.New("error", false)

	tmp1 := filepath.Join(dir, ".tmp-copy-abc123")
	tmp2 := filepath.Join(dir, "subdir", ".tmp-copy-def456")
	keep := filepath.Join(dir, "real-file.mkv")

	require.NoError(t, os.MkdirAll(filepath.Dir(tmp2), 0o755))
	require.NoError(t, os.WriteFile(tmp1, []byte("orphan"), 0o644))
	require.NoError(t, os.WriteFile(tmp2, []byte("orphan"), 0o644))
	require.NoError(t, os.WriteFile(keep, []byte("real"), 0o644))

	CleanupOrphanedTemps(dir, log)

	assert.NoFileExists(t, tmp1, "top-level orphan should be removed")
	assert.NoFileExists(t, tmp2, "nested orphan should be removed")
	assert.FileExists(t, keep, "real file must not be touched")
}

func TestCleanupOrphanedTemps_NoopOnEmptyRoot(t *testing.T) {
	// Must not panic or error when mediaRoot is empty.
	CleanupOrphanedTemps("", logger.New("error", false))
}

func TestCleanupOrphanedTemps_NoopOnMissingRoot(t *testing.T) {
	// Non-existent directory — WalkDir returns error on first entry, skipped silently.
	CleanupOrphanedTemps("/nonexistent/path/xyz", logger.New("error", false))
}

func TestCleanupOrphanedTemps_PreservesNonTempFiles(t *testing.T) {
	dir := t.TempDir()
	log := logger.New("error", false)

	files := []string{
		"movie.mkv",
		"tmp-copy-fake.mkv",   // similar name but no leading dot
		".tmp-other-file.txt", // dot-prefix but not .tmp-copy-
		"subdir/episode.mp4",
	}
	for _, f := range files {
		p := filepath.Join(dir, f)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte("data"), 0o644))
	}

	CleanupOrphanedTemps(dir, log)

	for _, f := range files {
		assert.FileExists(t, filepath.Join(dir, f), "should not remove %s", f)
	}
}
