package disk

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/darknessnerd/filebot-webui/internal/domain"
)

// Mount is a labelled path whose filesystem usage is reported.
type Mount struct {
	Label string
	Path  string
}

type Service struct {
	mounts []Mount
}

func New(mounts []Mount) *Service {
	return &Service{mounts: mounts}
}

// ParseMounts parses a comma-separated list of "label=path" or "path" entries.
// The media root is always first, labelled "media".
func ParseMounts(raw, mediaRoot string) []Mount {
	mounts := []Mount{{Label: "media", Path: mediaRoot}}
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		label, path, found := strings.Cut(entry, "=")
		if !found {
			path = label
			label = filepath.Base(filepath.Clean(path))
		}
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		mounts = append(mounts, Mount{Label: strings.TrimSpace(label), Path: filepath.Clean(path)})
	}
	return mounts
}

// Usage returns usage for every configured mount. Mounts that cannot be
// read are skipped; an error is returned only if none could be read.
func (s *Service) Usage(ctx context.Context) ([]domain.DiskUsage, error) {
	out := make([]domain.DiskUsage, 0, len(s.mounts))
	var firstErr error
	for _, m := range s.mounts {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("disk.Usage: %w", err)
		}
		var st syscall.Statfs_t
		if err := syscall.Statfs(m.Path, &st); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("disk.Usage %s: %w", m.Path, err)
			}
			continue
		}
		bsize := int64(st.Bsize)
		total := int64(st.Blocks) * bsize
		free := int64(st.Bavail) * bsize
		used := total - int64(st.Bfree)*bsize
		pct := 0
		// Match df: percent of space usable by unprivileged users.
		if used+free > 0 {
			pct = int(float64(used) / float64(used+free) * 100)
		}
		out = append(out, domain.DiskUsage{
			Label: m.Label, Path: m.Path,
			Total: total, Free: free, Used: used, UsedPercent: pct,
		})
	}
	if len(out) == 0 && firstErr != nil {
		return nil, firstErr
	}
	return out, nil
}
