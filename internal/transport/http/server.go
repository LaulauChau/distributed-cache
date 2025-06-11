package http

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/lachau/distributed-cache/internal/domain/cache"
)

type Server struct {
	server   *http.Server
	handlers *Handlers
	logger   *slog.Logger
}

type ServerConfig struct {
	Port         int
	CacheService cache.CacheService
	Logger       *slog.Logger
}

func NewServer(cfg ServerConfig) *Server {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	handlers := NewHandlers(Config{
		CacheService: cfg.CacheService,
		Logger:       cfg.Logger,
	})

	mux := http.NewServeMux()

	mux.HandleFunc("/cache/", handlers.cacheHandler)
	mux.HandleFunc("/health", handlers.HealthCheck)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      loggingMiddleware(cfg.Logger)(mux),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	return &Server{
		server:   server,
		handlers: handlers,
		logger:   cfg.Logger,
	}
}

func (s *Server) Start() error {
	s.logger.Info("starting HTTP server", "addr", s.server.Addr)
	return s.server.ListenAndServe()
}

func (s *Server) Stop(ctx context.Context) error {
	s.logger.Info("stopping HTTP server")
	return s.server.Shutdown(ctx)
}

func (h *Handlers) cacheHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.GetCache(w, r)
	case http.MethodPut:
		h.SetCache(w, r)
	case http.MethodDelete:
		h.DeleteCache(w, r)
	default:
		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed", "")
	}
}

func loggingMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			logger.Debug("incoming request",
				"method", r.Method,
				"path", r.URL.Path,
				"remote_addr", r.RemoteAddr,
				"user_agent", r.UserAgent(),
			)

			wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(wrapped, r)

			duration := time.Since(start)
			logger.Info("request completed",
				"method", r.Method,
				"path", r.URL.Path,
				"status", wrapped.statusCode,
				"duration_ms", duration.Milliseconds(),
				"remote_addr", r.RemoteAddr,
			)
		})
	}
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
