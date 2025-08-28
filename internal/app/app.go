package app

import (
	"context"
	"embed"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"webui-skeleton/internal/config"
	"webui-skeleton/internal/database"
	"webui-skeleton/internal/logger"
	"webui-skeleton/internal/server"
)

// Application represents the main application
type Application struct {
	config     *config.Config
	server     *server.Server
	db         *database.DB
	templateFS embed.FS
	staticFS   embed.FS
}

// New creates a new application instance
func New(templateFS embed.FS, staticFS embed.FS) *Application {
	return &Application{
		templateFS: templateFS,
		staticFS:   staticFS,
	}
}

// Initialize sets up all application components
func (app *Application) Initialize() error {
	if err := app.initConfig(); err != nil {
		return err
	}
	if err := app.initLogger(); err != nil {
		return err
	}
	logger.DisplayBanner()
	if err := app.initDatabase(); err != nil {
		return err
	}
	if err := app.migrateDatabase(); err != nil {
		return err
	}
	if err := app.initServer(); err != nil {
		return err
	}
	logger.Log.Info().Msg("✅ Application initialized successfully")
	return nil
}

func (app *Application) initConfig() error {
	var err error
	app.config, err = config.LoadConfiguration()
	if err != nil {
		logger.Log.Error().Err(err).Msg("❌ Failed to load configuration")
		return err
	}
	return nil
}

func (app *Application) initLogger() error {
	if app.config == nil {
		return fmt.Errorf("config is nil during logger initialization")
	}
	// Defensive: handle missing config values
	debug := app.config.Debug
	logLevel := app.config.LogLevel
	if logLevel == "" {
		logLevel = "info"
	}
	logger.Initialize(debug, logLevel)
	return nil
}

func (app *Application) initDatabase() error {
	if app.config == nil {
		return fmt.Errorf("config is nil during database initialization")
	}
	app.db = database.New(&app.config.Database)
	if err := app.db.Connect(); err != nil {
		logger.Log.Error().Err(err).Msg("❌ Failed to connect to database")
		return err
	}
	return nil
}

func (app *Application) migrateDatabase() error {
	if app.db == nil {
		return fmt.Errorf("database is nil during migration")
	}
	if err := app.db.Migrate(); err != nil {
		logger.Log.Error().Err(err).Msg("❌ Database migration failed")
		return err
	}
	return nil
}

func (app *Application) initServer() error {
	if app.config == nil || app.db == nil {
		return fmt.Errorf("missing config or db during server initialization")
	}
	app.server = server.New(app.config, app.templateFS, app.staticFS, app.db)
	app.server.SetupEngine()
	app.server.CreateHTTPServer()
	return nil
}

// Run starts the application
func (app *Application) Run() error {
	// Setup graceful shutdown
	ctx, cancel := setupGracefulShutdown()
	defer cancel()

	// Start the HTTP server (this blocks until shutdown)
	if err := app.server.Start(ctx); err != nil {
		logger.Log.Fatal().Err(err).Msg("❌ HTTP server failed")
		return err
	}

	logger.Log.Info().Msg("👋 Application stopped gracefully")
	return nil
}

// Cleanup performs cleanup operations
func (app *Application) Cleanup() {
	if app.db != nil {
		if err := app.db.Close(); err != nil {
			logger.Log.Error().Err(err).Msg("❌ Failed to close database connection")
		}
	}
	// Future: add cleanup for other resources (e.g., cache, file handles)
}

func setupGracefulShutdown() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())

	// Listen for interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		logger.Log.Info().Msg("🛑 Received shutdown signal")
		cancel()
	}()

	return ctx, cancel
}
