package handler

import (
	"context"
	"html/template"
	"net/http"

	"github.com/darknessnerd/filebot-webui/internal/domain"
	"github.com/darknessnerd/filebot-webui/internal/logger"
)

type diskService interface {
	Usage(ctx context.Context) ([]domain.DiskUsage, error)
}

type DiskHandler struct {
	svc  diskService
	tmpl *template.Template
	log  logger.Logger
}

func NewDiskHandler(svc diskService, tmpl *template.Template, log logger.Logger) *DiskHandler {
	return &DiskHandler{svc: svc, tmpl: tmpl, log: log}
}

// Widget renders the disk-usage partial used in the nav bar.
func (h *DiskHandler) Widget(w http.ResponseWriter, r *http.Request) {
	disks, err := h.svc.Usage(r.Context())
	if err != nil {
		h.log.Error().Err(err).Msg("DiskHandler.Widget")
		http.Error(w, "disk usage unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	if err := h.tmpl.ExecuteTemplate(w, "disk", map[string]any{"Disks": disks}); err != nil {
		h.log.Error().Err(err).Msg("disk partial render")
	}
}
