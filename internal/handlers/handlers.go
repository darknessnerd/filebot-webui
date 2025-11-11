package handlers

import (
	"github.com/gin-gonic/gin"
	"webui-skeleton/internal/auth"
	"webui-skeleton/internal/config"
	"webui-skeleton/internal/database"
	"webui-skeleton/internal/logger"
	"webui-skeleton/internal/models"
	"webui-skeleton/internal/repository"
)

// Handlers contains all handler instances
type Handlers struct {
	Auth        *AuthHandler
	API         *APIHandler
	Plex        *PlexHandler
	Deluge      *DelugeHandler
	Health      *HealthHandler
	FileBot     *FileBotHandler
	Directory   *DirectoryHandler
	Home        *HomeHandler
	config      *config.Config
	db          *database.DB
	authService *auth.Service
}

// NewHandlers creates a new handlers container with all handler instances
func NewHandlers(config *config.Config, db *database.DB, authService *auth.Service) *Handlers {
	// Create shared repositories
	delugeRepo := repository.NewDelugeServerRepository(db.DB)
	plexRepo := repository.NewPlexServerRepository(db.DB)

	// Initialize all handlers with their dependencies
	h := &Handlers{
		Plex:        NewPlexHandler(config, db, authService, plexRepo),
		Deluge:      NewDelugeHandler(config, db, authService, delugeRepo),
		Auth:        NewAuthHandler(config, db, authService),
		API:         NewAPIHandler(config, db, authService),
		Health:      NewHealthHandler(config),
		FileBot:     NewFileBotHandler(config, db, authService, delugeRepo, plexRepo),
		Directory:   NewDirectoryHandler(config, db, authService),
		Home:        NewHomeHandler(config, db, authService),
		config:      config,
		db:          db,
		authService: authService,
	}

	return h
}

// RenderWithHTMX renders either the content template (for HTMX) or base.html with content embedded
func RenderWithHTMX(c *gin.Context, contentTemplate string, data gin.H, requireAuth bool) {
	logger.Log.Debug().Str("contentTemplate", contentTemplate).Msg("RenderWithHTMX called")
	logger.Log.Debug().Interface("data", data).Msg("RenderWithHTMX: data map values")
	user := (*models.User)(nil)
	data["authenticated"] = false
	if requireAuth {
		userObj, exists := c.Get("user_obj")
		logger.Log.Debug().Bool("exists", exists).Interface("user_obj", userObj).Msg("RenderWithHTMX: user_obj from context")
		if !exists {
			logger.Log.Debug().Msg("RenderWithHTMX: user_obj not found, redirecting to /auth/login")
			c.Redirect(302, "/login")
			return
		}
		ok := false
		user, ok = userObj.(*models.User)
		logger.Log.Debug().Bool("ok", ok).Msg("RenderWithHTMX: user_obj type assertion to *models.User")
		if !ok {
			logger.Log.Debug().Msg("RenderWithHTMX: user_obj invalid type, redirecting to /auth/login")
			c.Redirect(302, "/login")
			return
		}
		var userID int
		var name, email, picture string
		var plex gin.H
		userID = user.ID
		name, email, picture, plex = extractUserInfo(user, user.Name, user.Email)

		userData := gin.H{
			"userID":        userID,
			"name":          name,
			"email":         email,
			"picture":       picture,
			"plex":          plex,
			"authenticated": true,
		}
		for k, v := range userData {
			data[k] = v
		}
	}

	if c.GetHeader("HX-Request") != "" {
		logger.Log.Debug().Str("render_type", "htmx").Str("template", contentTemplate).Msg("RenderWithHTMX: rendering HTMX partial")
		c.HTML(200, contentTemplate, data)
	} else {
		logger.Log.Debug().Str("render_type", "base").Str("template", "base.html").Str("partial", contentTemplate).Msg("RenderWithHTMX: rendering base.html with partial")
		// Render base.html and inject contentTemplate as partial
		// Gin's HTML rendering expects the base template to define a block for content
		// Pass the content template name as "content" key
		data["content"] = contentTemplate
		c.HTML(200, "base.html", data)
	}
}

// Helper to extract user info
func extractUserInfo(user *models.User, nameCtx, emailCtx string) (name, email, picture string, plex gin.H) {
	if user != nil {
		name = user.Name
		email = user.Email
		picture = user.Picture
		plex = gin.H{
			"username": user.PlexUsername,
			"email":    user.PlexEmail,
		}
	} else {
		name = nameCtx
		email = emailCtx
		picture = ""
		plex = nil
	}
	return
}
