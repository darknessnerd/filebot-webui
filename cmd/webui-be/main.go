package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/darknessnerd/filebot-webui/internal/config"
	"github.com/darknessnerd/filebot-webui/internal/logger"
	"github.com/darknessnerd/filebot-webui/internal/repository"
	"github.com/darknessnerd/filebot-webui/internal/service/auth"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	log := logger.New(cfg.LogLevel, cfg.Debug)

	db, err := repository.Open(cfg)
	if err != nil {
		log.Error().Err(err).Msg("failed to open database")
		os.Exit(1)
	}
	defer db.Close()

	if err := repository.RunMigrations(db); err != nil {
		log.Error().Err(err).Msg("failed to run migrations")
		os.Exit(1)
	}

	userRepo := repository.NewUserRepository(db)
	authSvc := auth.New(userRepo, cfg.JWTSecret, cfg.JWTExpiresIn, cfg.JWTIssuer, cfg.PlexClientID, log)
	_ = authSvc // used in Sprint 2 handlers

	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	addr := cfg.ServerHost + ":" + cfg.ServerPort
	log.Info().Str("addr", addr).Msg("starting server")
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Error().Err(err).Msg("server error")
		os.Exit(1)
	}
}
