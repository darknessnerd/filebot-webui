package server

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"time"
)

// setupRoutes configures all application routes
func (s *Server) setupRoutes() {
	// Add handlers to context for cross-handler functionality
	s.engine.Use(func(c *gin.Context) {
		// Make certain handlers available in the context for other handlers to access
		c.Set("deluge_service", s.handlers.Deluge)
		c.Next()
	})

	// Health check endpoints
	s.engine.GET("/health", s.handleHealth)
	s.engine.GET("/hh", s.handleHealth)

	// Authentication routes (unprotected and protected)
	s.setupAuthRoutes()

	// Web routes (protected)
	s.setupWebRoutes()

	// API routes (protected)
	s.setupAPIRoutes()

	// Static files (embedded)
	s.engine.GET("/static/*filepath", func(c *gin.Context) {
		file := c.Param("filepath")
		if file == "" || file == "/" {
			file = "/index.html"
		}
		// Prepend the embedded FS root so /static/css/styles.css maps to web/static/css/styles.css
		c.FileFromFS("web/static"+file, http.FS(s.staticFS))
	})
}

// setupAuthRoutes configures authentication routes
func (s *Server) setupAuthRoutes() {
	authGroup := s.engine.Group("/auth")
	{
		// Register Plex routes only if enabled/configured
		if s.config.Auth.AuthProvider == "plex" {
			authGroup.GET("/plex/login", s.handlers.Auth.PlexLogin)
			authGroup.GET("/plex/poll", s.handlers.Auth.PlexPoll)
			authGroup.GET("/plex/start", s.handlers.Auth.PlexStart)
			authGroup.GET("/plex/forward", s.handlers.Auth.PlexForward)
		}
		authGroup.POST("/logout", s.handlers.Auth.Logout)

		// Protected auth routes
		protected := authGroup.Group("")
		protected.Use(s.authService.AuthWithUserMiddleware())
		{
			protected.GET("/profile", s.handlers.Auth.GetProfile)
		}
	}
}

// setupWebRoutes configures web-related routes
func (s *Server) setupWebRoutes() {
	// Login page (no authentication required)
	s.engine.GET("/login", s.handlers.Auth.LoginPage)

	// Main application routes (require authentication)
	webGroup := s.engine.Group("/")
	webGroup.Use(s.authService.AuthWithUserMiddleware())
	{
		webGroup.GET("/", s.handlers.Home.HomePage)
		webGroup.GET("/plex/libraries/htmx", s.handlers.Plex.GetPlexLibraries)
		webGroup.GET("/plex/recently-added/htmx", s.handlers.Plex.RenderPlexRecentlyAddedHTMX)
		webGroup.GET("/plex/servers/htmx", s.handlers.Plex.RenderPlexServersHTMX)
		webGroup.POST("/plex/set-preferred-server", s.handlers.Plex.SetPreferredPlexServer)
		webGroup.GET("/plex/configure", s.handlers.Plex.RenderPlexConfigurePage)

		// Deluge routes
		webGroup.GET("/deluge", s.handlers.Deluge.RenderDelugeHomePage)
		webGroup.GET("/deluge/configure", s.handlers.Deluge.RenderDelugeConfigurePage)
		webGroup.GET("/deluge/servers/htmx", s.handlers.Deluge.RenderDelugeServersHTMX)
		webGroup.GET("/deluge/torrents/htmx", s.handlers.Deluge.GetTorrentsHTMX)
		webGroup.GET("/deluge/server-status/htmx", s.handlers.Deluge.GetServerStatusHTMX)
		webGroup.POST("/deluge/add-server", s.handlers.Deluge.AddDelugeServer)
		webGroup.POST("/deluge/set-preferred-server", s.handlers.Deluge.SetPreferredDelugeServer)
		webGroup.POST("/deluge/delete-server", s.handlers.Deluge.DeleteDelugeServer)
		webGroup.POST("/deluge/test-connection", s.handlers.Deluge.TestDelugeConnection)
		webGroup.POST("/deluge/add-torrent", s.handlers.Deluge.AddTorrent)
		webGroup.POST("/deluge/pause-torrent", s.handlers.Deluge.PauseTorrent)
		webGroup.POST("/deluge/resume-torrent", s.handlers.Deluge.ResumeTorrent)
		webGroup.POST("/deluge/remove-torrent", s.handlers.Deluge.RemoveTorrent)
		webGroup.POST("/deluge/pause-all", s.handlers.Deluge.PauseAllTorrents)
		webGroup.POST("/deluge/resume-all", s.handlers.Deluge.ResumeAllTorrents)
		webGroup.POST("/deluge/process-with-filebot", s.handlers.Deluge.ProcessCompletedTorrentWithFilebot)

		webGroup.GET("/filebot/form", s.handlers.FileBot.FileBotForm)
		webGroup.POST("/filebot/selection", s.handlers.FileBot.FileBotSelection)
		webGroup.POST("/filebot/execute", s.handlers.FileBot.FileBotExecute)
		// Directory browser routes
		webGroup.GET("/directory-browser", s.handlers.Directory.BrowseDirectory)
		// You can add POST endpoints for selection/collapse if needed

		// Add other web routes here
	}
}

// setupAPIRoutes configures API routes with authentication
func (s *Server) setupAPIRoutes() {
	apiGroup := s.engine.Group("/api/v1")

	// Public API routes
	apiGroup.GET("/status", s.handlers.API.Status)

	// Protected API routes
	protected := apiGroup.Group("")
	protected.Use(s.authService.AuthWithUserMiddleware())
	{
		protected.GET("/profile", s.handlers.Auth.GetProfile)
		protected.GET("/plex/libraries", s.handlers.Plex.GetPlexLibraries)
		protected.POST("/plex/libraries/:id/refresh", s.handlers.Plex.RefreshPlexLibrary)
	}
}

// handleHealth handles the health check endpoint
func (s *Server) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"version":   "1.0.0",
	})
}
