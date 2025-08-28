package main

import (
	"embed"
	"fmt"
	"os"

	"webui-skeleton/internal/app"
	"webui-skeleton/internal/logger"
)

//go:embed web/templates/*
var TemplateFS embed.FS

func main() {
	logger.Log.Info().Msg("Starting application...")
	// Log DIRECTORY_PRESETS value for debugging
	directoryPresets := os.Getenv("DIRECTORY_PRESETS")
	logger.Log.Info().Msgf("DIRECTORY_PRESETS value: %s", directoryPresets)
	if directoryPresets == "" {
		logger.Log.Error().Msg("Fatal: DIRECTORY_PRESETS environment variable must be set. Example: DIRECTORY_PRESETS=Downloads:/downloads,Media Library:/media")
		fmt.Fprintln(os.Stderr, "Fatal: DIRECTORY_PRESETS environment variable must be set. Example: DIRECTORY_PRESETS=Downloads:/downloads,Media Library:/media")
		os.Exit(1)
	}
	// Create and initialize application
	application := app.New(TemplateFS)
	defer application.Cleanup()

	// Initialize all components
	if err := application.Initialize(); err != nil {
		logger.Log.Error().Err(err).Msg("❌ Failed to initialize application")
		fmt.Fprintln(os.Stderr, "❌ Failed to initialize application:", err)
		os.Exit(1)
	}

	// Run the application
	if err := application.Run(); err != nil {
		logger.Log.Error().Err(err).Msg("❌ Application failed to run")
		fmt.Fprintln(os.Stderr, "❌ Application failed to run:", err)
		logger.Log.Error().Msgf("❌ Application exited with code: 1")
		os.Exit(1)
	}
}
