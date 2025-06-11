package http

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/lachau/distributed-cache/internal/domain/cache"
	"github.com/lachau/distributed-cache/internal/domain/errors"
)

type Handlers struct {
	cacheService cache.CacheService
	logger       *slog.Logger
}

type Config struct {
	CacheService cache.CacheService
	Logger       *slog.Logger
}

func NewHandlers(cfg Config) *Handlers {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	return &Handlers{
		cacheService: cfg.CacheService,
		logger:       cfg.Logger,
	}
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

type CacheResponse struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type HealthResponse struct {
	Status string   `json:"status"`
	Nodes  []string `json:"nodes"`
}

func (h *Handlers) GetCache(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed", "")
		return
	}

	key := h.extractKey(r.URL.Path)
	if key == "" {
		h.writeError(w, http.StatusBadRequest, "invalid key", "key cannot be empty")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	h.logger.Debug("handling get request", "key", key)

	value, err := h.cacheService.Get(ctx, key)
	if err != nil {
		h.handleServiceError(w, err, key)
		return
	}

	response := CacheResponse{
		Key:   key,
		Value: value,
	}

	h.writeJSON(w, http.StatusOK, response)
	h.logger.Debug("get request completed successfully", "key", key)
}

func (h *Handlers) SetCache(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed", "")
		return
	}

	key := h.extractKey(r.URL.Path)
	if key == "" {
		h.writeError(w, http.StatusBadRequest, "invalid key", "key cannot be empty")
		return
	}

	var req struct {
		Value string `json:"value"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}

	if req.Value == "" {
		h.writeError(w, http.StatusBadRequest, "invalid value", "value cannot be empty")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	h.logger.Debug("handling set request", "key", key)

	if err := h.cacheService.Set(ctx, key, req.Value); err != nil {
		h.handleServiceError(w, err, key)
		return
	}

	response := CacheResponse{
		Key:   key,
		Value: req.Value,
	}

	h.writeJSON(w, http.StatusCreated, response)
	h.logger.Debug("set request completed successfully", "key", key)
}

func (h *Handlers) DeleteCache(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed", "")
		return
	}

	key := h.extractKey(r.URL.Path)
	if key == "" {
		h.writeError(w, http.StatusBadRequest, "invalid key", "key cannot be empty")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	h.logger.Debug("handling delete request", "key", key)

	if err := h.cacheService.Delete(ctx, key); err != nil {
		h.handleServiceError(w, err, key)
		return
	}

	w.WriteHeader(http.StatusNoContent)
	h.logger.Debug("delete request completed successfully", "key", key)
}

func (h *Handlers) HealthCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed", "")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	h.logger.Debug("handling health check request")

	if err := h.cacheService.HealthCheck(ctx); err != nil {
		h.logger.Error("health check failed", "error", err)
		response := HealthResponse{
			Status: "unhealthy",
			Nodes:  h.cacheService.GetNodes(),
		}
		h.writeJSON(w, http.StatusServiceUnavailable, response)
		return
	}

	response := HealthResponse{
		Status: "healthy",
		Nodes:  h.cacheService.GetNodes(),
	}

	h.writeJSON(w, http.StatusOK, response)
	h.logger.Debug("health check completed successfully")
}

func (h *Handlers) extractKey(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) >= 2 && parts[0] == "cache" {
		return parts[1]
	}
	return ""
}

func (h *Handlers) handleServiceError(w http.ResponseWriter, err error, key string) {
	h.logger.Error("service error", "key", key, "error", err)

	switch err {
	case errors.ErrKeyNotFound:
		h.writeError(w, http.StatusNotFound, "key not found", "")
	case errors.ErrInvalidKey:
		h.writeError(w, http.StatusBadRequest, "invalid key", "")
	case errors.ErrInvalidValue:
		h.writeError(w, http.StatusBadRequest, "invalid value", "")
	case errors.ErrNoNodesAvailable:
		h.writeError(w, http.StatusServiceUnavailable, "no nodes available", "")
	case errors.ErrClientNotFound:
		h.writeError(w, http.StatusServiceUnavailable, "client not found", "")
	default:
		h.writeError(w, http.StatusInternalServerError, "internal server error", "")
	}
}

func (h *Handlers) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("failed to encode JSON response", "error", err)
	}
}

func (h *Handlers) writeError(w http.ResponseWriter, status int, error, message string) {
	response := ErrorResponse{
		Error:   error,
		Message: message,
	}
	h.writeJSON(w, status, response)
}
