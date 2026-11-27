package handler_test

import (
	"context"
	"errors"
	"html/template"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/darknessnerd/filebot-webui/internal/domain"
	"github.com/darknessnerd/filebot-webui/internal/handler"
	"github.com/darknessnerd/filebot-webui/internal/logger"
)

type stubDiskSvc struct {
	disks []domain.DiskUsage
	err   error
}

func (s stubDiskSvc) Usage(context.Context) ([]domain.DiskUsage, error) { return s.disks, s.err }

func diskTmpl(t *testing.T) *template.Template {
	t.Helper()
	return template.Must(template.New("").Parse(`{{define "disk"}}{{range .Disks}}{{.Label}}:{{.UsedPercent}}{{end}}{{end}}`))
}

func TestDiskHandler_Widget(t *testing.T) {
	log := logger.New("disabled", false)

	t.Run("ok", func(t *testing.T) {
		h := handler.NewDiskHandler(stubDiskSvc{disks: []domain.DiskUsage{{Label: "media", UsedPercent: 42}}}, diskTmpl(t), log)
		w := httptest.NewRecorder()
		h.Widget(w, httptest.NewRequest(http.MethodGet, "/disk", nil))
		require.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "media:42")
	})

	t.Run("service error -> 500", func(t *testing.T) {
		h := handler.NewDiskHandler(stubDiskSvc{err: errors.New("boom")}, diskTmpl(t), log)
		w := httptest.NewRecorder()
		h.Widget(w, httptest.NewRequest(http.MethodGet, "/disk", nil))
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}
