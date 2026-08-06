package anidb

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/darknessnerd/filebot-webui/internal/logger"
)

// IndexReloader is implemented by Client.  The scheduler calls it after each
// successful disk write so the live title index is updated without a restart.
type IndexReloader interface {
	ReloadIndex(data []byte) error
}

// SchedulerConfig holds tunable parameters for the titles refresh scheduler.
type SchedulerConfig struct {
	Interval         time.Duration // how often to wake up (default 12h)
	MinFetchInterval time.Duration // min time between remote downloads (default 24h)
	SourceURL        string
	TargetPath       string
}

// Scheduler runs a background loop that refreshes the AniDB titles file on a
// configurable interval while enforcing the AniDB-mandated 24h cooldown.
type Scheduler struct {
	cfg     SchedulerConfig
	log     logger.Logger
	reloader IndexReloader
	mu      sync.Mutex
}

// NewScheduler creates a Scheduler.  reloader may be nil (disk-only mode).
func NewScheduler(cfg SchedulerConfig, reloader IndexReloader, log logger.Logger) *Scheduler {
	if cfg.Interval <= 0 {
		cfg.Interval = 12 * time.Hour
	}
	if cfg.MinFetchInterval <= 0 {
		cfg.MinFetchInterval = 24 * time.Hour
	}
	return &Scheduler{cfg: cfg, reloader: reloader, log: log}
}

// Run starts the scheduler loop, performs an immediate check, then ticks every
// cfg.Interval.  Blocks until ctx is cancelled.
func (s *Scheduler) Run(ctx context.Context) {
	s.log.Info().
		Dur("interval", s.cfg.Interval).
		Dur("min_fetch_interval", s.cfg.MinFetchInterval).
		Str("target", s.cfg.TargetPath).
		Msg("anidb scheduler: started")

	s.tryRefresh(ctx)

	ticker := time.NewTicker(s.cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.log.Info().Msg("anidb scheduler: stopped")
			return
		case <-ticker.C:
			s.tryRefresh(ctx)
		}
	}
}

func (s *Scheduler) tryRefresh(ctx context.Context) {
	s.log.Debug().Msg("anidb scheduler: tick — checking freshness")

	if !s.mu.TryLock() {
		s.log.Warn().Msg("anidb scheduler: previous refresh still running, skipping tick")
		return
	}
	defer s.mu.Unlock()

	if age, fresh := s.isFresh(); fresh {
		s.log.Debug().
			Dur("age", age).
			Dur("min_fetch_interval", s.cfg.MinFetchInterval).
			Msg("anidb scheduler: titles file is fresh, skipping remote fetch")
		return
	}

	s.log.Info().
		Str("url", s.cfg.SourceURL).
		Str("target", s.cfg.TargetPath).
		Msg("anidb scheduler: fetching titles")

	content, err := RefreshTitlesFile(ctx, s.cfg.SourceURL, s.cfg.TargetPath, s.log)
	if err != nil {
		s.log.Error().Err(err).Msg("anidb scheduler: refresh failed")
		return
	}

	s.log.Debug().
		Int("bytes", len(content)).
		Str("path", s.cfg.TargetPath).
		Msg("anidb scheduler: disk write complete, reloading index")

	if s.reloader != nil {
		if err := s.reloader.ReloadIndex(content); err != nil {
			s.log.Error().Err(err).Msg("anidb scheduler: in-memory index reload failed")
			return
		}
	}

	s.log.Info().Str("path", s.cfg.TargetPath).Msg("anidb scheduler: refresh succeeded")
}

func (s *Scheduler) isFresh() (time.Duration, bool) {
	info, err := os.Stat(s.cfg.TargetPath)
	if err != nil {
		return 0, false
	}
	age := time.Since(info.ModTime())
	return age, age < s.cfg.MinFetchInterval
}
