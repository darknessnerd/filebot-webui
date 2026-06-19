package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/darknessnerd/filebot-webui/internal/config"
	"github.com/darknessnerd/filebot-webui/internal/handler"
	"github.com/darknessnerd/filebot-webui/internal/logger"
	"github.com/darknessnerd/filebot-webui/internal/repository"
	"github.com/darknessnerd/filebot-webui/internal/service/auth"
	"github.com/darknessnerd/filebot-webui/internal/service/deluge"
	"github.com/darknessnerd/filebot-webui/internal/service/filebot"
	"github.com/darknessnerd/filebot-webui/internal/service/plex"
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

	// Services
	userRepo := repository.NewUserRepository(db)
	authSvc := auth.New(userRepo, cfg.JWTSecret, cfg.JWTExpiresIn, cfg.JWTIssuer, cfg.PlexClientID, log)
	delugeSvc := deluge.NewClient(cfg.DelugeHost, cfg.DelugePort, cfg.DelugePassword, log)
	plexSvc := plex.NewClient(log)
	fbExecutor := filebot.NewExecutor(cfg.MediaRoot, cfg.FilebotPath, log)

	// Middleware
	authMiddleware := handler.RequireAuth(authSvc, userRepo, log)

	// Handlers
	authH := handler.NewAuthHandler(authSvc, cfg.PlexRedirectURL, log)
	torrentH := handler.NewTorrentHandler(delugeSvc, log)
	fileBotH := handler.NewFileBotHandler(fbExecutor, delugeSvc, plexSvc, log)

	mux := http.NewServeMux()

	// Public routes
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /login", authH.LoginPage)
	mux.HandleFunc("GET /auth/plex/start", authH.PlexStart)
	mux.HandleFunc("GET /auth/plex/forward", authH.PlexForward)
	mux.HandleFunc("POST /auth/logout", authH.Logout)

	// Protected routes
	mux.Handle("GET /", authMiddleware(http.HandlerFunc(torrentH.Dashboard)))
	mux.Handle("GET /torrents", authMiddleware(http.HandlerFunc(torrentH.List)))
	mux.Handle("GET /filebot", authMiddleware(http.HandlerFunc(fileBotH.Form)))
	mux.Handle("POST /filebot/execute", authMiddleware(http.HandlerFunc(fileBotH.Execute)))

	addr := cfg.ServerHost + ":" + cfg.ServerPort
	log.Info().Str("addr", addr).Msg("starting server")
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Error().Err(err).Msg("server error")
		os.Exit(1)
	}
}
