package cache

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/lachau/distributed-cache/internal/domain/cache"
	"github.com/lachau/distributed-cache/internal/domain/errors"
	"github.com/lachau/distributed-cache/internal/domain/hash"
)

type Service struct {
	hashRing hash.HashRing
	clients  map[string]cache.CacheRepository
	logger   *slog.Logger
}

type Config struct {
	Nodes   []string
	Clients map[string]cache.CacheRepository
	Logger  *slog.Logger
}

func NewService(cfg Config) (*Service, error) {
	if len(cfg.Nodes) == 0 {
		return nil, errors.ErrInvalidConfiguration
	}

	if len(cfg.Clients) == 0 {
		return nil, errors.ErrInvalidConfiguration
	}

	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	hashRing := hash.NewConsistentHashRing()
	for _, node := range cfg.Nodes {
		hashRing.AddNode(node)
	}

	service := &Service{
		hashRing: hashRing,
		clients:  cfg.Clients,
		logger:   cfg.Logger,
	}

	cfg.Logger.Info("cache service created successfully",
		"node_count", len(cfg.Nodes),
		"client_count", len(cfg.Clients))

	return service, nil
}

func (s *Service) Get(ctx context.Context, key string) (string, error) {
	if key == "" {
		return "", errors.ErrInvalidKey
	}

	node := s.hashRing.GetNode(key)
	if node == "" {
		s.logger.Error("no node available for key", "key", key)
		return "", errors.ErrNoNodesAvailable
	}

	client, exists := s.clients[node]
	if !exists {
		s.logger.Error("client not found for node", "node", node, "key", key)
		return "", errors.ErrClientNotFound
	}

	s.logger.Debug("routing get operation", "key", key, "node", node)

	value, err := client.Get(ctx, key)
	if err != nil {
		s.logger.Error("failed to get key from cache",
			"key", key, "node", node, "error", err)
		return "", err
	}

	s.logger.Debug("successfully retrieved key from cache", "key", key, "node", node)
	return value, nil
}

func (s *Service) Set(ctx context.Context, key, value string) error {
	if key == "" {
		return errors.ErrInvalidKey
	}
	if value == "" {
		return errors.ErrInvalidValue
	}

	node := s.hashRing.GetNode(key)
	if node == "" {
		s.logger.Error("no node available for key", "key", key)
		return errors.ErrNoNodesAvailable
	}

	client, exists := s.clients[node]
	if !exists {
		s.logger.Error("client not found for node", "node", node, "key", key)
		return errors.ErrClientNotFound
	}

	s.logger.Debug("routing set operation", "key", key, "node", node)

	if err := client.Set(ctx, key, value); err != nil {
		s.logger.Error("failed to set key in cache",
			"key", key, "node", node, "error", err)
		return err
	}

	s.logger.Debug("successfully set key in cache", "key", key, "node", node)
	return nil
}

func (s *Service) Delete(ctx context.Context, key string) error {
	if key == "" {
		return errors.ErrInvalidKey
	}

	node := s.hashRing.GetNode(key)
	if node == "" {
		s.logger.Error("no node available for key", "key", key)
		return errors.ErrNoNodesAvailable
	}

	client, exists := s.clients[node]
	if !exists {
		s.logger.Error("client not found for node", "node", node, "key", key)
		return errors.ErrClientNotFound
	}

	s.logger.Debug("routing delete operation", "key", key, "node", node)

	if err := client.Delete(ctx, key); err != nil {
		s.logger.Error("failed to delete key from cache",
			"key", key, "node", node, "error", err)
		return err
	}

	s.logger.Debug("successfully deleted key from cache", "key", key, "node", node)
	return nil
}

func (s *Service) HealthCheck(ctx context.Context) error {
	var lastErr error
	healthyClients := 0

	for node, client := range s.clients {
		if err := client.HealthCheck(ctx); err != nil {
			s.logger.Error("health check failed for client", "node", node, "error", err)
			lastErr = err
		} else {
			healthyClients++
			s.logger.Debug("health check passed for client", "node", node)
		}
	}

	if healthyClients == 0 {
		s.logger.Error("all clients failed health check")
		return fmt.Errorf("all clients unhealthy: %w", lastErr)
	}

	s.logger.Info("health check completed",
		"healthy_clients", healthyClients,
		"total_clients", len(s.clients))

	return nil
}

func (s *Service) GetNodes() []string {
	nodes := make([]string, 0, len(s.clients))
	for node := range s.clients {
		nodes = append(nodes, node)
	}
	return nodes
}
