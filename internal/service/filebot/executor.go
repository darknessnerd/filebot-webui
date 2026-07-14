package filebot

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/darknessnerd/filebot-webui/internal/domain"
	"github.com/darknessnerd/filebot-webui/internal/logger"
)

const shellMetachars = "$`;&|><\n\r"

type Executor struct {
	mediaRoot       string
	filebotPath     string
	log             logger.Logger
	allowedDB       map[string]bool
	allowedAction   map[string]bool
	allowedConflict map[string]bool
	allowedLog      map[string]bool
}

func NewExecutor(mediaRoot, filebotPath string, log logger.Logger) *Executor {
	return &Executor{
		mediaRoot:   filepath.Clean(mediaRoot),
		filebotPath: filebotPath,
		log:         log,
		allowedDB: map[string]bool{
			"TheMovieDB": true, "TheMovieDB::TV": true,
			"TheTVDB": true, "AniDB": true, "AcoustID": true, "OMDb": true,
		},
		allowedAction:   map[string]bool{"move": true, "copy": true, "symlink": true, "hardlink": true, "test": true},
		allowedConflict: map[string]bool{"skip": true, "replace": true, "auto": true, "index": true, "fail": true},
		allowedLog:      map[string]bool{"all": true, "fine": true, "info": true, "warning": true, "off": true},
	}
}

func (e *Executor) Execute(ctx context.Context, job domain.FileBotJob) (domain.FileBotResult, error) {
	if err := e.validate(job); err != nil {
		return domain.FileBotResult{}, err
	}

	args := e.buildArgs(job)
	e.log.Debug().Strs("args", args).Msg("filebot execute")
	e.log.Info().
		Str("action", job.Action).
		Str("db", job.DB).
		Int("source_count", len(job.SourcePaths)).
		Msg("filebot: starting job")

	cmd := exec.CommandContext(ctx, e.filebotPath, args...)
	out, err := cmd.CombinedOutput()

	result := domain.FileBotResult{RawOutput: string(out)}
	if err != nil {
		var exitErr *exec.ExitError
		// Exit code 3 means "no input files matched" — FileBot reports this after a
		// successful move when source files are already gone from disk. Treat as success
		// only for move; other actions have no post-move cleanup so exit 3 is unexpected.
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 3 && job.Action == "move" {
			e.log.Info().Str("action", job.Action).Msg("filebot: exit 3 (no input files) — treating as success after move")
			result.Successes = []string{string(out)}
			return result, nil
		}
		result.Errors = []string{fmt.Sprintf("filebot exited with error: %v\n%s", err, string(out))}
		e.log.Warn().Err(err).Str("action", job.Action).Msg("filebot: job failed")
		return result, fmt.Errorf("%w: %v", domain.ErrFileBotFailed, err)
	}
	result.Successes = []string{string(out)}
	e.log.Info().
		Str("action", job.Action).
		Int("source_count", len(job.SourcePaths)).
		Msg("filebot: job complete")
	return result, nil
}

func (e *Executor) validate(job domain.FileBotJob) error {
	if !e.allowedDB[job.DB] {
		return fmt.Errorf("%w: --db %q not allowed", domain.ErrInvalidArg, job.DB)
	}
	if !e.allowedAction[job.Action] {
		return fmt.Errorf("%w: --action %q not allowed", domain.ErrInvalidArg, job.Action)
	}
	if !e.allowedConflict[job.Conflict] {
		return fmt.Errorf("%w: --conflict %q not allowed", domain.ErrInvalidArg, job.Conflict)
	}
	if !e.allowedLog[job.LogLevel] {
		return fmt.Errorf("%w: --log %q not allowed", domain.ErrInvalidArg, job.LogLevel)
	}

	clean := filepath.Clean(job.Output)
	if clean != e.mediaRoot && !strings.HasPrefix(clean, e.mediaRoot+string(filepath.Separator)) {
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

func (e *Executor) buildArgs(job domain.FileBotJob) []string {
	args := []string{"-rename"}
	args = append(args, job.SourcePaths...)
	args = append(args,
		"--db", job.DB,
		"--action", job.Action,
		"--conflict", job.Conflict,
		"--log", job.LogLevel,
		"--output", job.Output,
		"-non-strict",
	)
	if job.Format != "" {
		args = append(args, "--format", job.Format)
	}
	if job.Filter != "" {
		args = append(args, "--filter", job.Filter)
	}
	if job.Query != "" {
		args = append(args, "--q", job.Query)
	}
	if job.Recursive {
		args = append(args, "-r")
	}
	return args
}

func rejectMetachars(argName, value string) error {
	if strings.ContainsAny(value, shellMetachars) {
		return fmt.Errorf("%w: %s contains shell metacharacters", domain.ErrInvalidArg, argName)
	}
	return nil
}
