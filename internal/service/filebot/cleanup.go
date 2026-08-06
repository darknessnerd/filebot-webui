package filebot

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/darknessnerd/filebot-webui/internal/logger"
)

// CleanupOrphanedTemps removes .tmp-copy-* files left in mediaRoot by
// copyFile calls that were interrupted before the final os.Rename.
// Called once at startup — safe to skip if mediaRoot is empty.
func CleanupOrphanedTemps(mediaRoot string, log logger.Logger) {
	if mediaRoot == "" {
		return
	}
	_ = filepath.WalkDir(mediaRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if strings.HasPrefix(d.Name(), ".tmp-copy-") {
			if removeErr := os.Remove(path); removeErr != nil {
				log.Warn().Err(removeErr).Str("path", path).Msg("startup: failed to remove orphaned temp file")
			} else {
				log.Info().Str("path", path).Msg("startup: removed orphaned temp file")
			}
		}
		return nil
	})
}
