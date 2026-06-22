package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/darknessnerd/filebot-webui/internal/config"
	"github.com/darknessnerd/filebot-webui/internal/handler"
	"github.com/darknessnerd/filebot-webui/internal/logger"
	"github.com/darknessnerd/filebot-webui/internal/repository"
	"github.com/darknessnerd/filebot-webui/internal/service/auth"
	"github.com/darknessnerd/filebot-webui/internal/service/deluge"
	"github.com/darknessnerd/filebot-webui/internal/service/filebot"
	"github.com/darknessnerd/filebot-webui/internal/service/plex"
)

//go:embed web/templates/* web/static/css/app.css
var webFS embed.FS

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

	// Parse templates with helper funcs
	funcMap := template.FuncMap{
		"formatBytes": formatBytes,
		"or": func(a, b string) string {
			if a != "" {
				return a
			}
			return b
		},
		"list": func(args ...string) []string { return args },
	}
	tmpl, err := template.New("").Funcs(funcMap).ParseFS(webFS,
		"web/templates/base.html",
		"web/templates/dashboard.html",
		"web/templates/torrents.html",
		"web/templates/filebot_form.html",
		"web/templates/filebot_result.html",
	)
	if err != nil {
		log.Error().Err(err).Msg("failed to parse templates")
		os.Exit(1)
	}

	// FILEBOT_LICENSE_PATH: register license once at boot if configured
	if cfg.FilebotLicensePath != "" {
		registerCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		out, regErr := exec.CommandContext(registerCtx, cfg.FilebotPath, "--license", cfg.FilebotLicensePath).CombinedOutput()
		cancel()
		if regErr != nil {
			log.Warn().Err(regErr).Str("output", string(out)).Msg("filebot license registration failed — continuing")
		} else {
			log.Info().Msg("filebot license registered")
		}
	}

	// Services
	userRepo := repository.NewUserRepository(db)
	authSvc := auth.New(userRepo, cfg.JWTSecret, cfg.JWTExpiresIn, cfg.JWTIssuer, cfg.PlexClientID, log)
	delugeSvc := deluge.NewClient(cfg.DelugeHost, cfg.DelugePort, cfg.DelugePassword, log)
	plexSvc := plex.NewClient(log)
	fbExecutor := filebot.NewExecutor(cfg.MediaRoot, cfg.FilebotPath, log)

	// Middleware
	authMiddleware := handler.RequireAuth(authSvc, userRepo, log)

	// Handlers (now template-aware)
	authH := handler.NewAuthHandler(authSvc, cfg.PlexRedirectURL, log)
	torrentH := handler.NewTorrentHandler(delugeSvc, tmpl, log)
	fileBotH := handler.NewFileBotHandler(fbExecutor, delugeSvc, plexSvc, tmpl, cfg.MediaRoot, log)

	mux := http.NewServeMux()

	// Static files — strip the "web/" prefix so /static/css/app.css maps correctly
	staticFS, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Error().Err(err).Msg("failed to sub static FS")
		os.Exit(1)
	}
	mux.Handle("GET /static/", http.FileServer(http.FS(staticFS)))

	// Public routes
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /login", func(w http.ResponseWriter, r *http.Request) {
		data, err := webFS.ReadFile("web/templates/login.html")
		if err != nil {
			http.Error(w, "not found", 404)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write(data)
	})
	mux.HandleFunc("GET /auth/plex/start", authH.PlexStart)
	mux.HandleFunc("GET /auth/plex/forward", authH.PlexForward)
	mux.HandleFunc("POST /auth/logout", authH.Logout)

	// Protected routes
	mux.Handle("GET /", authMiddleware(http.HandlerFunc(torrentH.Dashboard)))
	mux.Handle("GET /torrents", authMiddleware(http.HandlerFunc(torrentH.List)))
	mux.Handle("GET /filebot", authMiddleware(http.HandlerFunc(fileBotH.Form)))
	mux.Handle("POST /filebot/execute", authMiddleware(http.HandlerFunc(fileBotH.Execute)))

	addr := cfg.ServerHost + ":" + cfg.ServerPort
	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	sigCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Info().Str("addr", addr).Msg("starting server")
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			log.Error().Err(err).Msg("server error")
			os.Exit(1)
		}
	}()

	<-sigCtx.Done()
	log.Info().Msg("shutting down")

	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		log.Error().Err(err).Msg("graceful shutdown failed")
	}
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
