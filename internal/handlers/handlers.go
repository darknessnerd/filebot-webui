package handlers

import (
	"github.com/gin-gonic/gin"
	"os"
	"path/filepath"
	"webui-skeleton/internal/auth"
	"webui-skeleton/internal/config"
	"webui-skeleton/internal/database"
	"webui-skeleton/internal/logger"
	"webui-skeleton/internal/models"
	"webui-skeleton/internal/repository"
)

// Handlers contains all handler instances
type Handlers struct {
	Home        *HomeHandler
	Auth        *AuthHandler
	API         *APIHandler
	Plex        *PlexHandler
	Health      *HealthHandler
	config      *config.Config
	db          *database.DB
	authService *auth.Service
}

// NewHandlers creates a new handlers container with all handler instances
func NewHandlers(config *config.Config, db *database.DB, authService *auth.Service) *Handlers {
	return &Handlers{
		Home:        NewHomeHandler(config, db, authService),
		Plex:        NewPlexHandler(config, db, authService, repository.NewPlexServerRepository(db.DB)),
		Auth:        NewAuthHandler(config, db, authService),
		API:         NewAPIHandler(config, db, authService),
		Health:      NewHealthHandler(config),
		config:      config,
		db:          db,
		authService: authService,
	}
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

// DirectoryBrowserHandler handles directory browsing and selection
// Add to Handlers struct if needed
func (h *Handlers) DirectoryBrowser(c *gin.Context) {
	// Get path from query or default
	rootDirs := []string{"."} // You can customize this
	rootPath := rootDirs[0]
	currentPath := c.Query("path")
	if currentPath == "" {
		currentPath = rootPath
	}
	parentPath := filepath.Dir(currentPath)
	if parentPath == "." || parentPath == "/" {
		parentPath = rootPath
	}

	// Collapse state (simple query param for demo)
	isCollapsed := c.DefaultQuery("collapsed", "false") == "true"

	// Selection state (simple query param for demo)
	selectedItems := map[string]bool{}
	if sel := c.QueryArray("selected"); len(sel) > 0 {
		for _, s := range sel {
			selectedItems[s] = true
		}
	}

	// List files and directories
	entries, err := os.ReadDir(currentPath)
	files := []map[string]string{}
	fileCount := 0
	dirCount := 0
	for _, entry := range entries {
		itemType := "file"
		if entry.IsDir() {
			itemType = "directory"
			dirCount++
		} else {
			fileCount++
		}
		files = append(files, map[string]string{
			"Name": entry.Name(),
			"Type": itemType,
			"Path": filepath.Join(currentPath, entry.Name()),
		})
	}

	// Selection mode (for demo, can be from query)
	selectionMode := c.DefaultQuery("selectionMode", "both")

	// Is current directory selected
	isSelectedDirectory := selectedItems[filepath.Base(currentPath)]

	// Helper for template
	isCheckboxAvailable := func(item map[string]string) bool {
		return selectionMode == "both" ||
			(selectionMode == "files" && item["Type"] == "file") ||
			(selectionMode == "directories" && item["Type"] == "directory")
	}
	isSelected := func(item map[string]string) bool {
		return selectedItems[item["Name"]]
	}

	// Render template
	RenderWithHTMX(c, "directory_browser.html", gin.H{
		"CurrentPath":          currentPath,
		"RootPath":             rootPath,
		"ParentPath":           parentPath,
		"AvailableDirectories": rootDirs,
		"Files":                files,
		"FileCount":            fileCount,
		"DirectoryCount":       dirCount,
		"SelectionMode":        selectionMode,
		"IsCollapsed":          isCollapsed,
		"IsSelectedDirectory":  isSelectedDirectory,
		"IsCheckboxAvailable":  isCheckboxAvailable,
		"IsSelected":           isSelected,
		"Loading":              false,
		"Error":                err,
	}, true)
}
