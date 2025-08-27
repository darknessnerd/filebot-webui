package server

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"time"
)

// setupRoutes configures all application routes
func (s *Server) setupRoutes() {
	// Health check endpoints
	s.engine.GET("/health", s.handleHealth)
	s.engine.GET("/hh", s.handleHealth)

	// Authentication routes (unprotected and protected)
	s.setupAuthRoutes()

	// Web routes (protected)
	s.setupWebRoutes()

	// API routes (protected)
	s.setupAPIRoutes()

	// Static files
	s.engine.Static("/static", "cmd/webui-be/web/static")
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
		webGroup.GET("/dashboard", s.handlers.Home.DashboardPage)
		webGroup.GET("/plex/libraries/htmx", s.handlers.Plex.GetPlexLibraries)
		webGroup.GET("/plex/recently-added/htmx", s.handlers.Plex.RenderPlexRecentlyAddedHTMX)
		webGroup.GET("/plex/servers/htmx", s.handlers.Plex.RenderPlexServersHTMX)
		webGroup.POST("/plex/set-preferred-server", s.handlers.Plex.SetPreferredPlexServer)
		webGroup.GET("/plex/configure", s.handlers.Plex.RenderPlexConfigurePage)
		webGroup.GET("/filebot/form", s.handlers.FileBotForm)
		webGroup.POST("/filebot/execute-filebot", s.handlers.FileBotExecute)
		webGroup.POST("/filebot/selection", s.handlers.FileBotSelection)
		// Directory browser routes
		webGroup.GET("/directory-browser", s.handlers.DirectoryBrowser)
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
