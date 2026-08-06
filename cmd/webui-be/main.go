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
	"os/signal"
	"syscall"
	"time"

	"github.com/darknessnerd/filebot-webui/internal/config"
	"github.com/darknessnerd/filebot-webui/internal/domain"
	"github.com/darknessnerd/filebot-webui/internal/handler"
	"github.com/darknessnerd/filebot-webui/internal/logger"
	"github.com/darknessnerd/filebot-webui/internal/repository"
	"github.com/darknessnerd/filebot-webui/internal/service/anidb"
	"github.com/darknessnerd/filebot-webui/internal/service/auth"
	"github.com/darknessnerd/filebot-webui/internal/service/deluge"
	"github.com/darknessnerd/filebot-webui/internal/service/filebot"
	"github.com/darknessnerd/filebot-webui/internal/service/plex"
	"github.com/darknessnerd/filebot-webui/internal/service/tmdb"
)

//go:embed web/templates/* web/static/css/app.css web/static/css/icons.css web/static/fonts
var webFS embed.FS

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	log := logger.New(cfg.LogLevel, cfg.Debug)

	filebot.CleanupOrphanedTemps(cfg.MediaRoot, log)

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
	tmpFiles := []string{
		"web/templates/base.html",
		"web/templates/dashboard.html",
		"web/templates/torrents.html",
		"web/templates/filebot_form.html",
		"web/templates/filebot_result.html",
		"web/templates/filebot_not_found.html",
	}
	if cfg.Debug {
		tmpFiles = append(tmpFiles, "web/templates/playground.html")
	}
	tmpl, err := template.New("").Funcs(funcMap).ParseFS(webFS, tmpFiles...)
	if err != nil {
		log.Error().Err(err).Msg("failed to parse templates")
		os.Exit(1)
	}

	// Services
	userRepo := repository.NewUserRepository(db)
	authSvc := auth.New(userRepo, cfg.JWTSecret, cfg.JWTExpiresIn, cfg.JWTIssuer, cfg.PlexClientID, log)

	var (
		delugeSvc interface {
			ListCompleted(ctx context.Context) ([]domain.Torrent, error)
			DeleteTorrent(ctx context.Context, id string) error
		}
		plexSvc interface {
			RefreshLibraries(ctx context.Context, plexToken string) error
		}
		authMiddleware func(http.Handler) http.Handler
	)

	var (
		movieResolver interface {
			SearchMovie(ctx context.Context, query string, year int) (*domain.MovieMatch, error)
		}
		tvResolver interface {
			SearchTV(ctx context.Context, query string, year int) (*domain.TVMatch, error)
		}
		animeResolver interface {
			SearchAnime(ctx context.Context, query string, year int) (*domain.AnimeMatch, error)
			SearchAnimeByAID(ctx context.Context, aid int) (*domain.AnimeMatch, error)
		}
		hasTMDBProvider bool
	)

	var titleScheduler *anidb.Scheduler

	// Metadata resolvers always wire from real config.
	if cfg.TMDBAccessToken != "" {
		tmdbClient := tmdb.NewClient(cfg.TMDBAccessToken, log)
		movieResolver = tmdbClient
		tvResolver = tmdbClient
		hasTMDBProvider = true
	} else {
		log.Warn().Msg("TMDB_ACCESS_TOKEN not configured: TMDB providers disabled")
	}

	anidbClient := anidb.NewClient(cfg.AniDBClient, cfg.AniDBClientVer, cfg.AniDBProtoVer, cfg.AniDBBaseURL, log)

	startupRefresh := cfg.AniDBRefreshTitlesOnStart && cfg.AniDBTitlesURL != "" && cfg.AniDBTitlesFile != ""
	if startupRefresh {
		content, err := anidb.RefreshTitlesFile(context.Background(), cfg.AniDBTitlesURL, cfg.AniDBTitlesFile, log)
		if err != nil {
			log.Warn().Err(err).Msg("anidb: titles refresh on start failed")
			if err := anidbClient.LoadIndexFromFile(cfg.AniDBTitlesFile); err != nil {
				log.Warn().Err(err).Msg("anidb: could not load titles file from disk")
			}
		} else if err := anidbClient.ReloadIndex(content); err != nil {
			log.Warn().Err(err).Msg("anidb: in-memory index reload after startup refresh failed")
		}
	} else if cfg.AniDBTitlesFile != "" {
		if err := anidbClient.LoadIndexFromFile(cfg.AniDBTitlesFile); err != nil {
			log.Warn().Err(err).Msg("anidb: could not load titles file from disk")
		}
	}

	if cfg.AniDBSchedulerEnabled && cfg.AniDBTitlesURL != "" && cfg.AniDBTitlesFile != "" {
		titleScheduler = anidb.NewScheduler(anidb.SchedulerConfig{
			Interval:         cfg.AniDBSchedulerInterval,
			MinFetchInterval: cfg.AniDBMinFetchInterval,
			SourceURL:        cfg.AniDBTitlesURL,
			TargetPath:       cfg.AniDBTitlesFile,
		}, anidbClient, log)
	}

	animeResolver = anidbClient

	// DevMode mocks only Deluge, Plex, and auth — metadata resolvers above stay real.
	if cfg.DevMode {
		log.Warn().Msg("DEV_MODE enabled — Deluge/Plex mocked, auth bypassed")
		delugeSvc = &mockDelugeClient{}
		plexSvc = &mockPlexClient{}
		authMiddleware = func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				next.ServeHTTP(w, r.WithContext(handler.WithUser(r.Context(), devUser)))
			})
		}
	} else {
		delugeSvc = deluge.NewClient(cfg.DelugeHost, cfg.DelugePort, cfg.DelugePassword, log)
		plexSvc = plex.NewClient(log)
		authMiddleware = handler.RequireAuth(authSvc, userRepo, log)
	}

	fbResolver := filebot.NewResolver(movieResolver, tvResolver, animeResolver)
	fbExecutor := filebot.NewInternal(cfg.MediaRoot, fbResolver, log)

	// Handlers (now template-aware)
	authH := handler.NewAuthHandler(authSvc, cfg.PlexRedirectURL, log)
	torrentH := handler.NewTorrentHandler(delugeSvc, tmpl, log, cfg.Debug)
	fileBotH := handler.NewFileBotHandler(fbExecutor, delugeSvc, plexSvc, tmpl, cfg.MediaRoot, hasTMDBProvider, log)
	plexH := handler.NewPlexHandler(plexSvc, log)

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

	// Playground — only in debug mode, localhost only
	if cfg.Debug {
		mux.HandleFunc("GET /playground", func(w http.ResponseWriter, r *http.Request) {
			host := r.Host
			if host != "localhost" && host != "localhost:"+cfg.ServerPort && host != "127.0.0.1" && host != "127.0.0.1:"+cfg.ServerPort {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			if err := tmpl.ExecuteTemplate(w, "playground", nil); err != nil {
				http.Error(w, err.Error(), 500)
			}
		})
		log.Info().Msg("CSS playground enabled at /playground (debug mode)")
	}

	// Protected routes
	mux.Handle("GET /", authMiddleware(http.HandlerFunc(torrentH.Dashboard)))
	mux.Handle("GET /torrents", authMiddleware(http.HandlerFunc(torrentH.List)))
	mux.Handle("GET /filebot", authMiddleware(http.HandlerFunc(fileBotH.Form)))
	mux.Handle("POST /filebot/execute", authMiddleware(http.HandlerFunc(fileBotH.Execute)))
	mux.Handle("POST /plex/refresh", authMiddleware(http.HandlerFunc(plexH.Refresh)))

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

	if titleScheduler != nil {
		go titleScheduler.Run(sigCtx)
	}

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
