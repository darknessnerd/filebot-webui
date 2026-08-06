package anidb

import (
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/darknessnerd/filebot-webui/internal/logger"
)

// validXML is a minimal well-formed AniDB titles payload.
const validXML = `<?xml version="1.0"?><animetitles><anime aid="1"><title>Test</title></anime></animetitles>`

func noopLogger() logger.Logger { return logger.New("error", false) }

// TestScheduler_SkipsWhenFresh verifies that no HTTP request is made when the
// local file was modified within MinFetchInterval.
func TestScheduler_SkipsWhenFresh(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(validXML))
	}))
	defer srv.Close()

	dir := t.TempDir()
	target := filepath.Join(dir, "anime-titles.xml")
	// Write the file with mtime = now (definitely fresh).
	require.NoError(t, os.WriteFile(target, []byte(validXML), 0o644))

	s := NewScheduler(SchedulerConfig{
		Interval:         10 * time.Millisecond,
		MinFetchInterval: 24 * time.Hour,
		SourceURL:        srv.URL,
		TargetPath:       target,
	}, nil, noopLogger())

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	s.Run(ctx)

	assert.Equal(t, int32(0), calls.Load(), "expected no HTTP calls when file is fresh")
}

// TestScheduler_FetchesWhenStale verifies that a remote download happens when
// the file is older than MinFetchInterval.
func TestScheduler_FetchesWhenStale(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(validXML))
	}))
	defer srv.Close()

	dir := t.TempDir()
	target := filepath.Join(dir, "anime-titles.xml")
	require.NoError(t, os.WriteFile(target, []byte(validXML), 0o644))
	// Make the file appear old.
	old := time.Now().Add(-25 * time.Hour)
	require.NoError(t, os.Chtimes(target, old, old))

	s := NewScheduler(SchedulerConfig{
		Interval:         10 * time.Millisecond,
		MinFetchInterval: 24 * time.Hour,
		SourceURL:        srv.URL,
		TargetPath:       target,
	}, nil, noopLogger())

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	s.Run(ctx)

	assert.GreaterOrEqual(t, calls.Load(), int32(1), "expected at least one HTTP call when file is stale")
}

// TestScheduler_FetchesWhenMissing verifies that a remote download happens
// when the target file does not exist yet.
func TestScheduler_FetchesWhenMissing(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(validXML))
	}))
	defer srv.Close()

	target := filepath.Join(t.TempDir(), "anime-titles.xml")

	s := NewScheduler(SchedulerConfig{
		Interval:         10 * time.Millisecond,
		MinFetchInterval: 24 * time.Hour,
		SourceURL:        srv.URL,
		TargetPath:       target,
	}, nil, noopLogger())

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	s.Run(ctx)

	assert.GreaterOrEqual(t, calls.Load(), int32(1))
	data, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Contains(t, string(data), "<animetitles")
}

// TestScheduler_AtomicReplaceGzip verifies that a gzip-compressed payload is
// correctly decompressed and atomically written.
func TestScheduler_AtomicReplaceGzip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		var buf bytes.Buffer
		zw := gzip.NewWriter(&buf)
		_, _ = zw.Write([]byte(validXML))
		_ = zw.Close()
		w.Header().Set("Content-Encoding", "gzip")
		_, _ = w.Write(buf.Bytes())
	}))
	defer srv.Close()

	target := filepath.Join(t.TempDir(), "anime-titles.xml")

	s := NewScheduler(SchedulerConfig{
		Interval:         10 * time.Millisecond,
		MinFetchInterval: 24 * time.Hour,
		SourceURL:        srv.URL,
		TargetPath:       target,
	}, nil, noopLogger())

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	s.Run(ctx)

	data, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Contains(t, string(data), "<animetitles")
	assert.NoFileExists(t, target+".tmp", "temp file must not remain after atomic replace")
}

// TestScheduler_RejectsInvalidPayload verifies that a payload without the
// expected <animetitles marker is rejected and the target file is not written.
func TestScheduler_RejectsInvalidPayload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><body>not anime titles</body></html>`))
	}))
	defer srv.Close()

	target := filepath.Join(t.TempDir(), "anime-titles.xml")

	s := NewScheduler(SchedulerConfig{
		Interval:         10 * time.Millisecond,
		MinFetchInterval: 24 * time.Hour,
		SourceURL:        srv.URL,
		TargetPath:       target,
	}, nil, noopLogger())

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	s.Run(ctx)

	assert.NoFileExists(t, target, "target must not exist after invalid payload")
}

// TestScheduler_HandlesHTTPFailure verifies that a non-200 response is
// handled gracefully without a panic.
func TestScheduler_HandlesHTTPFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	target := filepath.Join(t.TempDir(), "anime-titles.xml")

	s := NewScheduler(SchedulerConfig{
		Interval:         10 * time.Millisecond,
		MinFetchInterval: 24 * time.Hour,
		SourceURL:        srv.URL,
		TargetPath:       target,
	}, nil, noopLogger())

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// Must not panic.
	assert.NotPanics(t, func() { s.Run(ctx) })
	assert.NoFileExists(t, target)
}

// TestScheduler_NoOverlapOnConcurrentTicks verifies that concurrent tryRefresh
// calls do not result in overlapping downloads.  We simulate overlap by
// making the server slow and forcing two ticks close together.
func TestScheduler_NoOverlapOnConcurrentTicks(t *testing.T) {
	var active atomic.Int32
	var overlap atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if active.Add(1) > 1 {
			overlap.Store(true)
		}
		// Simulate slow download.
		time.Sleep(60 * time.Millisecond)
		active.Add(-1)
		_, _ = w.Write([]byte(validXML))
	}))
	defer srv.Close()

	target := filepath.Join(t.TempDir(), "anime-titles.xml")

	s := NewScheduler(SchedulerConfig{
		Interval:         5 * time.Millisecond,
		MinFetchInterval: 0, // always download
		SourceURL:        srv.URL,
		TargetPath:       target,
	}, nil, noopLogger())

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	s.Run(ctx)

	assert.False(t, overlap.Load(), "concurrent downloads must not overlap")
}

// TestScheduler_ContextCancellation verifies that Run returns promptly when
// the context is cancelled.
func TestScheduler_ContextCancellation(t *testing.T) {
	target := filepath.Join(t.TempDir(), "anime-titles.xml")
	// Write a fresh file so no download is attempted.
	require.NoError(t, os.WriteFile(target, []byte(validXML), 0o644))

	s := NewScheduler(SchedulerConfig{
		Interval:         10 * time.Second, // long interval — should never fire
		MinFetchInterval: 24 * time.Hour,
		SourceURL:        "http://localhost:0",
		TargetPath:       target,
	}, nil, noopLogger())

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		s.Run(ctx)
		close(done)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return within 2s after context cancellation")
	}
}

// TestScheduler_DefaultIntervals verifies that zero-value intervals get
// sensible defaults applied.
func TestScheduler_DefaultIntervals(t *testing.T) {
	s := NewScheduler(SchedulerConfig{}, nil, noopLogger())
	assert.Equal(t, 12*time.Hour, s.cfg.Interval)
	assert.Equal(t, 24*time.Hour, s.cfg.MinFetchInterval)
}

// stubReloader records ReloadIndex calls for assertion.
type stubReloader struct {
	calls atomic.Int32
	last  []byte
}

func (r *stubReloader) ReloadIndex(data []byte) error {
	r.calls.Add(1)
	r.last = data
	return nil
}

// TestScheduler_CallsReloaderOnSuccess verifies that ReloadIndex is called
// with the downloaded content after a successful disk write.
func TestScheduler_CallsReloaderOnSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(validXML))
	}))
	defer srv.Close()

	target := filepath.Join(t.TempDir(), "anime-titles.xml")
	reloader := &stubReloader{}

	s := NewScheduler(SchedulerConfig{
		Interval:         10 * time.Millisecond,
		MinFetchInterval: 24 * time.Hour,
		SourceURL:        srv.URL,
		TargetPath:       target,
	}, reloader, noopLogger())

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	s.Run(ctx)

	assert.GreaterOrEqual(t, reloader.calls.Load(), int32(1), "ReloadIndex must be called after successful fetch")
	assert.Contains(t, string(reloader.last), "<animetitles")
}
