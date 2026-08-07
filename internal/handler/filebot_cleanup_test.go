package handler

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/darknessnerd/filebot-webui/internal/logger"
)

func TestRemoveEmptyDirs_RemovesFullyEmptyTree(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "Season 1")
	require.NoError(t, os.Mkdir(sub, 0755))

	require.NoError(t, removeEmptyDirs(root))

	_, err := os.Stat(root)
	assert.True(t, os.IsNotExist(err), "root dir should be removed when fully empty")
}

func TestRemoveEmptyDirs_KeepsNonEmptyDir(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "Season 1")
	require.NoError(t, os.Mkdir(sub, 0755))
	f, err := os.Create(filepath.Join(sub, "leftover.nfo"))
	require.NoError(t, err)
	f.Close()

	require.NoError(t, removeEmptyDirs(root))

	_, err = os.Stat(root)
	assert.NoError(t, err, "root dir should survive when a file remains")
}

func TestRemoveEmptyDirs_NestedMixedTree(t *testing.T) {
	// root/
	//   empty-sub/          ← should be removed
	//   nonempty-sub/
	//     file.txt          ← blocks removal of nonempty-sub and root
	root := t.TempDir()
	emptySub := filepath.Join(root, "empty-sub")
	require.NoError(t, os.Mkdir(emptySub, 0755))
	nonEmptySub := filepath.Join(root, "nonempty-sub")
	require.NoError(t, os.Mkdir(nonEmptySub, 0755))
	f, err := os.Create(filepath.Join(nonEmptySub, "file.txt"))
	require.NoError(t, err)
	f.Close()

	require.NoError(t, removeEmptyDirs(root))

	_, err = os.Stat(emptySub)
	assert.True(t, os.IsNotExist(err), "empty-sub should be removed")
	_, err = os.Stat(nonEmptySub)
	assert.NoError(t, err, "nonempty-sub should survive")
	_, err = os.Stat(root)
	assert.NoError(t, err, "root should survive when nonempty-sub remains")
}

func TestRemoveSourceDir_NoopOnFile(t *testing.T) {
	root := t.TempDir()
	f, err := os.Create(filepath.Join(root, "movie.mkv"))
	require.NoError(t, err)
	f.Close()
	filePath := filepath.Join(root, "movie.mkv")

	h := &FileBotHandler{log: logger.New("error", false)}
	// Must not error or remove the file.
	h.removeSourceDir(filePath)

	_, err = os.Stat(filePath)
	assert.NoError(t, err, "file should not be touched")
}

func TestRemoveSourceDir_NoopOnMissing(t *testing.T) {
	h := &FileBotHandler{log: logger.New("error", false)}
	// Must not panic on a path that doesn't exist.
	h.removeSourceDir("/nonexistent/path/does/not/exist")
}
