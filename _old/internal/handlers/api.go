package handlers

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"webui-skeleton/internal/auth"
	"webui-skeleton/internal/config"
	"webui-skeleton/internal/database"
)

// APIHandler handles API endpoints
type APIHandler struct {
	config  *config.Config
	db      *database.DB
	authSvc *auth.Service
}

// NewAPIHandler creates a new API handler
func NewAPIHandler(config *config.Config, db *database.DB, authSvc *auth.Service) *APIHandler {
	return &APIHandler{
		config:  config,
		db:      db,
		authSvc: authSvc,
	}
}

// Status returns API status information
func (h *APIHandler) Status(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"version": "1.0.0",
		"api":     "v1",
		"app":     "webui-skeleton",
	})
}
