package filebot

// Filesystem operation tests: real files on disk, no metadata resolver needed.
// Covers applyAction (all actions + all conflict modes), moveFile, and copyFile.
//
// Scenario matrix:
//
//	Test                                   Function     Action    Conflict  Setup                    Asserts
//	─────────────────────────────────────  ───────────  ────────  ────────  ───────────────────────  ──────────────────────────────────────────
//	ApplyAction_Move_TargetAbsent          applyAction  move      skip      source only              source gone; target has content
//	ApplyAction_Copy_TargetAbsent          applyAction  copy      skip      source only              source stays; target has content
//	ApplyAction_Symlink_TargetAbsent       applyAction  symlink   skip      source only              symlink at target → source
//	ApplyAction_Hardlink_TargetAbsent      applyAction  hardlink  skip      source only              same inode at source and target
//	ApplyAction_Test_NoMutation            applyAction  test      skip      source only              source untouched; no target created
//	Conflict_Skip_TargetExists             applyAction  move      skip      source + target          source unchanged; target unchanged; SKIP msg
//	Conflict_Replace_TargetExists          applyAction  move      replace   source + target          source gone; target has source content
//	Conflict_Index_TargetExists            applyAction  move      index     source + target          source gone; target(2) created; original target intact
//	Conflict_Auto_TargetExists             applyAction  move      auto      source + target          same as index
//	Conflict_Fail_TargetExists             applyAction  move      fail      source + target          error returned; both files intact
//	MoveFile_ContentIntegrity              moveFile     —         —         source with known bytes  target content matches; source gone
//	CopyFile_ContentIntegrity              copyFile     —         —         source with known bytes  target content matches; source intact
//	CopyFile_MkdirAll_CreatesParentDirs    applyAction  copy      skip      source; deep target      parent dirs created at 0755
//	ApplyAction_Move_MkdirAll             applyAction  move      skip      source; deep target      parent dirs created; file at target

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testContent = "sentinel-payload-12345"

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(b)
}

// ── applyAction: all actions, target absent ────────────────────────────────

func TestApplyAction_Move_TargetAbsent(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.mkv")
	dst := filepath.Join(dir, "out", "dst.mkv")
	writeFile(t, src, testContent)

	msg, err := applyAction("move", "skip", src, dst)

	require.NoError(t, err)
	assert.Contains(t, msg, "MOVE")
	assert.FileExists(t, dst)
	assert.Equal(t, testContent, readFile(t, dst))
	assert.NoFileExists(t, src)
}

func TestApplyAction_Copy_TargetAbsent(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.mkv")
	dst := filepath.Join(dir, "out", "dst.mkv")
	writeFile(t, src, testContent)

	msg, err := applyAction("copy", "skip", src, dst)

	require.NoError(t, err)
	assert.Contains(t, msg, "COPY")
	assert.FileExists(t, dst)
	assert.Equal(t, testContent, readFile(t, dst))
	assert.FileExists(t, src, "source must survive a copy")
}

func TestApplyAction_Symlink_TargetAbsent(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.mkv")
	dst := filepath.Join(dir, "out", "dst.mkv")
	writeFile(t, src, testContent)

	msg, err := applyAction("symlink", "skip", src, dst)

	require.NoError(t, err)
	assert.Contains(t, msg, "SYMLINK")
	info, serr := os.Lstat(dst)
	require.NoError(t, serr)
	assert.True(t, info.Mode()&os.ModeSymlink != 0, "dst must be a symlink")
	target, lerr := os.Readlink(dst)
	require.NoError(t, lerr)
	assert.Equal(t, src, target)
}

func TestApplyAction_Hardlink_TargetAbsent(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.mkv")
	dst := filepath.Join(dir, "out", "dst.mkv")
	writeFile(t, src, testContent)

	msg, err := applyAction("hardlink", "skip", src, dst)

	require.NoError(t, err)
	assert.Contains(t, msg, "HARDLINK")
	srcInfo, _ := os.Stat(src)
	dstInfo, _ := os.Stat(dst)
	// Same inode = real hardlink.
	assert.Equal(t, srcInfo.Sys(), dstInfo.Sys())
}

func TestApplyAction_Test_NoMutation(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.mkv")
	dst := filepath.Join(dir, "out", "dst.mkv")
	writeFile(t, src, testContent)

	msg, err := applyAction("test", "skip", src, dst)

	require.NoError(t, err)
	assert.Contains(t, msg, "TEST")
	assert.FileExists(t, src, "source must not be touched")
	assert.NoFileExists(t, dst, "target must not be created")
}

// ── conflict modes ─────────────────────────────────────────────────────────

func TestConflict_Skip_TargetExists(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.mkv")
	dst := filepath.Join(dir, "dst.mkv")
	writeFile(t, src, "source-content")
	writeFile(t, dst, "original-content")

	msg, err := applyAction("move", "skip", src, dst)

	require.NoError(t, err)
	assert.Contains(t, msg, "SKIP")
	assert.FileExists(t, src, "source must survive when skipped")
	assert.Equal(t, "original-content", readFile(t, dst), "target must be unchanged")
}

func TestConflict_Replace_TargetExists(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.mkv")
	dst := filepath.Join(dir, "dst.mkv")
	writeFile(t, src, "new-content")
	writeFile(t, dst, "old-content")

	_, err := applyAction("move", "replace", src, dst)

	require.NoError(t, err)
	assert.NoFileExists(t, src)
	assert.Equal(t, "new-content", readFile(t, dst))
}

func TestConflict_Index_TargetExists(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.mkv")
	dst := filepath.Join(dir, "dst.mkv")
	writeFile(t, src, "new-content")
	writeFile(t, dst, "original-content")

	msg, err := applyAction("move", "index", src, dst)

	require.NoError(t, err)
	assert.NoFileExists(t, src)
	// Indexed file must exist.
	indexed := filepath.Join(dir, "dst (2).mkv")
	assert.FileExists(t, indexed)
	assert.Equal(t, "new-content", readFile(t, indexed))
	// Original target untouched.
	assert.Equal(t, "original-content", readFile(t, dst))
	assert.Contains(t, msg, "dst (2).mkv")
}

func TestConflict_Auto_TargetExists(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.mkv")
	dst := filepath.Join(dir, "dst.mkv")
	writeFile(t, src, "new-content")
	writeFile(t, dst, "original-content")

	_, err := applyAction("move", "auto", src, dst)

	require.NoError(t, err)
	assert.NoFileExists(t, src)
	assert.FileExists(t, filepath.Join(dir, "dst (2).mkv"))
	assert.Equal(t, "original-content", readFile(t, dst))
}

func TestConflict_Fail_TargetExists(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.mkv")
	dst := filepath.Join(dir, "dst.mkv")
	writeFile(t, src, "source-content")
	writeFile(t, dst, "original-content")

	_, err := applyAction("move", "fail", src, dst)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "target exists")
	// Both files intact.
	assert.FileExists(t, src)
	assert.Equal(t, "original-content", readFile(t, dst))
}

// ── moveFile / copyFile content integrity ──────────────────────────────────

func TestMoveFile_ContentIntegrity(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.mkv")
	dst := filepath.Join(dir, "dst.mkv")
	writeFile(t, src, testContent)

	require.NoError(t, moveFile(src, dst))

	assert.Equal(t, testContent, readFile(t, dst))
	assert.NoFileExists(t, src)
}

func TestCopyFile_ContentIntegrity(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.mkv")
	dst := filepath.Join(dir, "dst.mkv")
	writeFile(t, src, testContent)

	require.NoError(t, copyFile(src, dst))

	assert.Equal(t, testContent, readFile(t, dst))
	assert.FileExists(t, src, "source must survive copy")
}

// ── parent directory creation ──────────────────────────────────────────────

func TestApplyAction_Copy_MkdirAll_CreatesParentDirs(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.mkv")
	dst := filepath.Join(dir, "a", "b", "c", "dst.mkv")
	writeFile(t, src, testContent)

	_, err := applyAction("copy", "skip", src, dst)

	require.NoError(t, err)
	assert.FileExists(t, dst)
	info, serr := os.Stat(filepath.Join(dir, "a", "b"))
	require.NoError(t, serr)
	assert.Equal(t, os.FileMode(0755), info.Mode().Perm())
}

func TestApplyAction_Move_MkdirAll_CreatesParentDirs(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.mkv")
	dst := filepath.Join(dir, "x", "y", "z", "dst.mkv")
	writeFile(t, src, testContent)

	_, err := applyAction("move", "skip", src, dst)

	require.NoError(t, err)
	assert.FileExists(t, dst)
	assert.NoFileExists(t, src)
	// Verify deep parent exists with correct perms.
	info, serr := os.Stat(filepath.Join(dir, "x", "y", "z"))
	require.NoError(t, serr)
	assert.Equal(t, os.FileMode(0755), info.Mode().Perm())
}

// ── index collision: multiple existing indexed files ───────────────────────

func TestConflict_Index_MultipleCollisions(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.mkv")
	writeFile(t, src, "newest")
	writeFile(t, filepath.Join(dir, "dst.mkv"), "v1")
	writeFile(t, filepath.Join(dir, "dst (2).mkv"), "v2")
	writeFile(t, filepath.Join(dir, "dst (3).mkv"), "v3")

	dst := filepath.Join(dir, "dst.mkv")
	_, err := applyAction("move", "index", src, dst)

	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(dir, "dst (4).mkv"))
	assert.Equal(t, "newest", readFile(t, filepath.Join(dir, "dst (4).mkv")))
	// All prior versions intact.
	for _, f := range []string{"dst.mkv", "dst (2).mkv", "dst (3).mkv"} {
		assert.FileExists(t, filepath.Join(dir, f))
	}
}

// ── large-ish content: copyFile handles multi-chunk reads correctly ─────────

func TestCopyFile_LargeContent(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.mkv")
	dst := filepath.Join(dir, "dst.mkv")
	// 512 KiB of repeated pattern — exercises the io.Copy loop.
	content := strings.Repeat("abcdefgh", 64*1024)
	writeFile(t, src, content)

	require.NoError(t, copyFile(src, dst))

	assert.Equal(t, content, readFile(t, dst))
}
