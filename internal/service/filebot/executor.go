package filebot

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/darknessnerd/filebot-webui/internal/domain"
	"github.com/darknessnerd/filebot-webui/internal/logger"
)

const shellMetachars = "$`;&|><\n\r"

type Engine interface {
	Execute(ctx context.Context, job domain.FileBotJob) (domain.FileBotResult, error)
}

type Service struct {
	mediaRoot       string
	engine          Engine
	log             logger.Logger
	allowedDB       map[string]bool
	allowedAction   map[string]bool
	allowedConflict map[string]bool
	allowedLog      map[string]bool
}

func New(mediaRoot string, engine Engine, log logger.Logger) *Service {
	return &Service{
		mediaRoot: filepath.Clean(mediaRoot),
		engine:    engine,
		log:       log,
		allowedDB: map[string]bool{
			"TheMovieDB": true, "TheMovieDB::TV": true,
			"TheTVDB": true, "AniDB": true, "AcoustID": true, "OMDb": true,
		},
		allowedAction:   map[string]bool{"move": true, "copy": true, "symlink": true, "hardlink": true, "test": true},
		allowedConflict: map[string]bool{"skip": true, "replace": true, "auto": true, "index": true, "fail": true},
		allowedLog:      map[string]bool{"all": true, "fine": true, "info": true, "warning": true, "off": true},
	}
}

func (s *Service) Execute(ctx context.Context, job domain.FileBotJob) (domain.FileBotResult, error) {
	if err := s.validate(job); err != nil {
		return domain.FileBotResult{}, err
	}

	s.log.Info().
		Str("action", job.Action).
		Str("db", job.DB).
		Int("source_count", len(job.SourcePaths)).
		Msg("filebot: starting job")

	result, err := s.engine.Execute(ctx, job)
	if err != nil {
		s.log.Warn().Err(err).Str("action", job.Action).Msg("filebot: job failed")
		return result, err
	}

	s.log.Info().
		Str("action", job.Action).
		Int("source_count", len(job.SourcePaths)).
		Msg("filebot: job complete")
	return result, nil
}

func (s *Service) validate(job domain.FileBotJob) error {
	if !s.allowedDB[job.DB] {
		return fmt.Errorf("%w: --db %q not allowed", domain.ErrInvalidArg, job.DB)
	}
	if !s.allowedAction[job.Action] {
		return fmt.Errorf("%w: --action %q not allowed", domain.ErrInvalidArg, job.Action)
	}
	if !s.allowedConflict[job.Conflict] {
		return fmt.Errorf("%w: --conflict %q not allowed", domain.ErrInvalidArg, job.Conflict)
	}
	if !s.allowedLog[job.LogLevel] {
		return fmt.Errorf("%w: --log %q not allowed", domain.ErrInvalidArg, job.LogLevel)
	}

	clean := filepath.Clean(job.Output)
	if clean != s.mediaRoot && !strings.HasPrefix(clean, s.mediaRoot+string(filepath.Separator)) {
		return fmt.Errorf("%w: --output %q is outside MEDIA_ROOT", domain.ErrInvalidArg, job.Output)
	}

	for i, p := range job.SourcePaths {
		if strings.ContainsAny(p, shellMetachars) {
			return fmt.Errorf("%w: source_paths[%d] contains shell metacharacters", domain.ErrInvalidArg, i)
		}
	}

	if err := rejectMetachars("--format", job.Format); err != nil {
		return err
	}
	if err := rejectMetachars("--filter", job.Filter); err != nil {
		return err
	}
	if err := rejectMetachars("--q", job.Query); err != nil {
		return err
	}
	return nil
}

func rejectMetachars(argName, value string) error {
	if strings.ContainsAny(value, shellMetachars) {
		return fmt.Errorf("%w: %s contains shell metacharacters", domain.ErrInvalidArg, argName)
	}
	return nil
}
