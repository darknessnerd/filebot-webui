package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"webui-skeleton/internal/config"
)

// HealthHandler handles health check endpoints
type HealthHandler struct {
	config *config.Config
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(config *config.Config) *HealthHandler {
	return &HealthHandler{
		config: config,
	}
}

// HealthCheck returns the health status of the application
func (h *HealthHandler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"app":       "webui-skeleton",
		"version":   "1.0.0",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"uptime":    "running",
	})
}
