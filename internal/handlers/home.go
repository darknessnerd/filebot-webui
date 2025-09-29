// filepath: /home/crinitib/workspace-personal/filebot-webui/internal/handlers/home.go
package handlers

import (
	"github.com/gin-gonic/gin"
	"webui-skeleton/internal/auth"
	"webui-skeleton/internal/config"
	"webui-skeleton/internal/database"
)

// HomeHandler manages home page rendering
type HomeHandler struct {
	config      *config.Config
	db          *database.DB
	authService *auth.Service
}

// NewHomeHandler creates a new home page handler
func NewHomeHandler(config *config.Config, db *database.DB, authService *auth.Service) *HomeHandler {
	return &HomeHandler{
		config:      config,
		db:          db,
		authService: authService,
	}
}

// HomePage renders the main home page
func (h *HomeHandler) HomePage(c *gin.Context) {
	RenderWithHTMX(c, "home.html", gin.H{
		"title": "FileBot WebUI - Home",
	}, true)
}
