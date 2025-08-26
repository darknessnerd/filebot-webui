package main

import (
	"embed"

	"webui-skeleton/internal/app"
	"webui-skeleton/internal/logger"
)

//go:embed web/templates/*
var TemplateFS embed.FS

func main() {
	// Create and initialize application
	application := app.New(TemplateFS)
	defer application.Cleanup()

	// Initialize all components
	if err := application.Initialize(); err != nil {
		logger.Log.Fatal().Err(err).Msg("❌ Failed to initialize application")
	}

	// Run the application
	if err := application.Run(); err != nil {
		logger.Log.Fatal().Err(err).Msg("❌ Application failed to run")
	}
}
