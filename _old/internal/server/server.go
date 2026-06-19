package server

import (
	"context"
	"embed"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"html/template"
	"webui-skeleton/internal/auth"
	"webui-skeleton/internal/config"
	"webui-skeleton/internal/database"
	"webui-skeleton/internal/handlers"
	"webui-skeleton/internal/logger"
)

type Server struct {
	config      *config.Config
	templateFS  embed.FS
	staticFS    embed.FS
	db          *database.DB
	authService *auth.Service
	handlers    *handlers.Handlers
	engine      *gin.Engine
	httpServer  *http.Server
}

// New creates a new server instance
func New(config *config.Config, templateFS embed.FS, staticFS embed.FS, db *database.DB) *Server {
	return &Server{
		config:     config,
		templateFS: templateFS,
		staticFS:   staticFS,
		db:         db,
	}
}

// JoinSelectedFiles joins selected file paths for template usage
func joinSelectedFiles(files []map[string]string) string {
	paths := []string{}
	for _, f := range files {
		if path, ok := f["Path"]; ok {
			paths = append(paths, path)
		}
	}
	return strings.Join(paths, ",")
}

// SetupEngine configures the Gin engine with routes and middleware
func (s *Server) SetupEngine() {
	// Set Gin mode based on debug flag
	if !s.config.Debug {
		gin.SetMode(gin.ReleaseMode)
	}

	// Create Gin engine
	s.engine = gin.New()

	// Add basic middleware
	s.engine.Use(gin.Logger())
	s.engine.Use(gin.Recovery())

	// Load HTML templates using htmlx SetHTMLTemplate
	s.setupTemplates()
	// Setup authentication service
	logger.Log.Debug().Msg("Setting up authentication service")
	s.authService = auth.NewService(
		s.db.DB,
		s.config.Auth.JWTSecret,
		s.config.Auth.JWTExpiresIn,
		s.config.Auth.JWTIssuer,
		s.config.Auth.PlexClientID,
		s.config.Auth.PlexClientSecret,
		s.config.Auth.PlexRedirectURL,
	)

	// Initialize handlers
	logger.Log.Debug().Msg("Initializing handlers")
	s.handlers = handlers.NewHandlers(s.config, s.db, s.authService)

	// Setup routes
	logger.Log.Debug().Msg("Setting up routes")
	s.setupRoutes()

	logger.Log.Info().Msg("✅ Server engine configured")
}

// setupTemplates configures HTML template rendering
func (s *Server) setupTemplates() {
	funcMap := map[string]interface{}{
		"joinSelectedFiles": joinSelectedFiles,
		"split":             strings.Split,
		"sub":               func(a, b int) int { return a - b },
		"add":               func(a, b int) int { return a + b },
		"splitPath":         func(p string) []string { return strings.Split(p, "/") },
		"joinPath":          func(parts []string) string { return strings.Join(parts, "/") },
		"formatBytes": func(bytes int64) string {
			const unit = 1024
			if bytes < unit {
				return fmt.Sprintf("%d B", bytes)
			}
			div, exp := int64(unit), 0
			for n := bytes / unit; n >= unit; n /= unit {
				div *= unit
				exp++
			}
			return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
		},
		"formatETA": func(eta int64) string {
			if eta < 0 {
				return "∞"
			}
			hours := eta / 3600
			minutes := (eta % 3600) / 60
			seconds := eta % 60
			if hours > 0 {
				return fmt.Sprintf("%dh %dm", hours, minutes)
			} else if minutes > 0 {
				return fmt.Sprintf("%dm %ds", minutes, seconds)
			} else {
				return fmt.Sprintf("%ds", seconds)
			}
		},
	}

	tmpl := template.Must(template.New("").Funcs(funcMap).ParseFS(s.templateFS,
		"web/templates/base.html",
		"web/templates/landing_page.html",
		"web/templates/home.html",
		"web/templates/login.html",
		"web/templates/plex/plex_libraries.html",
		"web/templates/plex/plex_servers.html",
		"web/templates/plex/plex_recently_added.html",
		"web/templates/plex/plex_configure.html",
		"web/templates/deluge/deluge_configure.html",
		"web/templates/deluge/deluge_servers.html",
		"web/templates/deluge/deluge_home.html",
		"web/templates/deluge/deluge_torrents.html",
		"web/templates/deluge/deluge_server_status.html",
		"web/templates/directory_browser.html",
		"web/templates/filebot_form.html",
		"web/templates/directory_browser_embedded.html",
	))

	s.engine.SetHTMLTemplate(tmpl)
}

// CreateHTTPServer creates the HTTP server instance
func (s *Server) CreateHTTPServer() {
	addr := fmt.Sprintf("%s:%d", s.config.Server.Host, s.config.Server.Port)

	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      s.engine,
		ReadTimeout:  60 * time.Minute,
		WriteTimeout: 60 * time.Minute,
		IdleTimeout:  60 * time.Minute,
	}

	logger.Log.Info().
		Str("address", addr).
		Msg("HTTP server configured")
}

// Start starts the HTTP server
func (s *Server) Start(ctx context.Context) error {
	// Start server in a goroutine
	go func() {
		logger.Log.Info().
			Str("address", s.httpServer.Addr).
			Msg("🚀 Starting HTTP server")

		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Log.Fatal().Err(err).Msg("❌ HTTP server failed to start")
		}
	}()

	// Wait for shutdown signal
	<-ctx.Done()

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Log.Error().Err(err).Msg("❌ HTTP server graceful shutdown failed")
		return err
	}
	if shutdownErr := ctx.Err(); shutdownErr != nil && shutdownErr != context.Canceled {
		logger.Log.Warn().Err(shutdownErr).Msg("Shutdown reason (context error)")
	}
	logger.Log.Info().Msg("🛑 HTTP server stopped gracefully")
	return nil
}
