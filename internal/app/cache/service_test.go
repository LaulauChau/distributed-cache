package cache

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/lachau/distributed-cache/internal/domain/cache"
	domainErrors "github.com/lachau/distributed-cache/internal/domain/errors"
)

type mockCacheRepository struct {
	data        map[string]string
	getErr      error
	setErr      error
	deleteErr   error
	healthErr   error
	getCalls    int
	setCalls    int
	deleteCalls int
	healthCalls int
}

func newMockCacheRepository() *mockCacheRepository {
	return &mockCacheRepository{
		data: make(map[string]string),
	}
}

func (m *mockCacheRepository) Get(ctx context.Context, key string) (string, error) {
	m.getCalls++
	if m.getErr != nil {
		return "", m.getErr
	}
	value, exists := m.data[key]
	if !exists {
		return "", domainErrors.ErrKeyNotFound
	}
	return value, nil
}

func (m *mockCacheRepository) Set(ctx context.Context, key, value string) error {
	m.setCalls++
	if m.setErr != nil {
		return m.setErr
	}
	m.data[key] = value
	return nil
}

func (m *mockCacheRepository) Delete(ctx context.Context, key string) error {
	m.deleteCalls++
	if m.deleteErr != nil {
		return m.deleteErr
	}
	delete(m.data, key)
	return nil
}

func (m *mockCacheRepository) HealthCheck(ctx context.Context) error {
	m.healthCalls++
	return m.healthErr
}

func TestNewService(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		expectError bool
		errorType   error
	}{
		{
			name: "valid configuration",
			config: Config{
				Nodes: []string{"node1", "node2"},
				Clients: map[string]cache.CacheRepository{
					"node1": newMockCacheRepository(),
					"node2": newMockCacheRepository(),
				},
				Logger: slog.Default(),
			},
			expectError: false,
		},
		{
			name: "empty nodes",
			config: Config{
				Nodes: []string{},
				Clients: map[string]cache.CacheRepository{
					"node1": newMockCacheRepository(),
				},
			},
			expectError: true,
			errorType:   domainErrors.ErrInvalidConfiguration,
		},
		{
			name: "empty clients",
			config: Config{
				Nodes:   []string{"node1"},
				Clients: map[string]cache.CacheRepository{},
			},
			expectError: true,
			errorType:   domainErrors.ErrInvalidConfiguration,
		},
		{
			name: "nil logger defaults to slog.Default",
			config: Config{
				Nodes: []string{"node1"},
				Clients: map[string]cache.CacheRepository{
					"node1": newMockCacheRepository(),
				},
				Logger: nil,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, err := NewService(tt.config)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				if tt.errorType != nil && !errors.Is(err, tt.errorType) {
					t.Errorf("expected error %v, got %v", tt.errorType, err)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if service == nil {
				t.Error("expected service to be created")
			}
		})
	}
}

func TestService_Get(t *testing.T) {
	mock1 := newMockCacheRepository()
	mock2 := newMockCacheRepository()

	service, err := NewService(Config{
		Nodes: []string{"node1", "node2"},
		Clients: map[string]cache.CacheRepository{
			"node1": mock1,
			"node2": mock2,
		},
		Logger: slog.Default(),
	})
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	ctx := context.Background()

	t.Run("successful get", func(t *testing.T) {
		key := "test-key"
		value := "test-value"

		err := service.Set(ctx, key, value)
		if err != nil {
			t.Fatalf("failed to set up test: %v", err)
		}

		retrievedValue, err := service.Get(ctx, key)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if retrievedValue != value {
			t.Errorf("expected '%s', got '%s'", value, retrievedValue)
		}
	})

	t.Run("key not found", func(t *testing.T) {
		_, err := service.Get(ctx, "nonexistent-key")
		if !errors.Is(err, domainErrors.ErrKeyNotFound) {
			t.Errorf("expected ErrKeyNotFound, got %v", err)
		}
	})

	t.Run("empty key", func(t *testing.T) {
		_, err := service.Get(ctx, "")
		if !errors.Is(err, domainErrors.ErrInvalidKey) {
			t.Errorf("expected ErrInvalidKey, got %v", err)
		}
	})

	t.Run("client error", func(t *testing.T) {
		mock1.getErr = errors.New("client error")
		mock2.getErr = errors.New("client error")
		defer func() {
			mock1.getErr = nil
			mock2.getErr = nil
		}()

		_, err := service.Get(ctx, "test-key")
		if err == nil {
			t.Error("expected error but got none")
		}
	})
}

func TestService_Set(t *testing.T) {
	mock1 := newMockCacheRepository()
	mock2 := newMockCacheRepository()

	service, err := NewService(Config{
		Nodes: []string{"node1", "node2"},
		Clients: map[string]cache.CacheRepository{
			"node1": mock1,
			"node2": mock2,
		},
		Logger: slog.Default(),
	})
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	ctx := context.Background()

	t.Run("successful set", func(t *testing.T) {
		err := service.Set(ctx, "test-key", "test-value")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if mock1.setCalls == 0 && mock2.setCalls == 0 {
			t.Error("expected at least one set call")
		}
	})

	t.Run("empty key", func(t *testing.T) {
		err := service.Set(ctx, "", "test-value")
		if !errors.Is(err, domainErrors.ErrInvalidKey) {
			t.Errorf("expected ErrInvalidKey, got %v", err)
		}
	})

	t.Run("empty value", func(t *testing.T) {
		err := service.Set(ctx, "test-key", "")
		if !errors.Is(err, domainErrors.ErrInvalidValue) {
			t.Errorf("expected ErrInvalidValue, got %v", err)
		}
	})

	t.Run("client error", func(t *testing.T) {
		mock1.setErr = errors.New("client error")
		mock2.setErr = errors.New("client error")
		defer func() {
			mock1.setErr = nil
			mock2.setErr = nil
		}()

		err := service.Set(ctx, "test-key", "test-value")
		if err == nil {
			t.Error("expected error but got none")
		}
	})
}

func TestService_Delete(t *testing.T) {
	mock1 := newMockCacheRepository()
	mock2 := newMockCacheRepository()

	service, err := NewService(Config{
		Nodes: []string{"node1", "node2"},
		Clients: map[string]cache.CacheRepository{
			"node1": mock1,
			"node2": mock2,
		},
		Logger: slog.Default(),
	})
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	ctx := context.Background()

	t.Run("successful delete", func(t *testing.T) {
		key := "test-key"
		value := "test-value"

		err := service.Set(ctx, key, value)
		if err != nil {
			t.Fatalf("failed to set up test: %v", err)
		}

		err = service.Delete(ctx, key)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if mock1.deleteCalls == 0 && mock2.deleteCalls == 0 {
			t.Error("expected at least one delete call")
		}
	})

	t.Run("empty key", func(t *testing.T) {
		err := service.Delete(ctx, "")
		if !errors.Is(err, domainErrors.ErrInvalidKey) {
			t.Errorf("expected ErrInvalidKey, got %v", err)
		}
	})

	t.Run("client error", func(t *testing.T) {
		mock1.deleteErr = errors.New("client error")
		mock2.deleteErr = errors.New("client error")
		defer func() {
			mock1.deleteErr = nil
			mock2.deleteErr = nil
		}()

		err := service.Delete(ctx, "test-key")
		if err == nil {
			t.Error("expected error but got none")
		}
	})
}

func TestService_HealthCheck(t *testing.T) {
	t.Run("all clients healthy", func(t *testing.T) {
		mock1 := newMockCacheRepository()
		mock2 := newMockCacheRepository()

		service, err := NewService(Config{
			Nodes: []string{"node1", "node2"},
			Clients: map[string]cache.CacheRepository{
				"node1": mock1,
				"node2": mock2,
			},
			Logger: slog.Default(),
		})
		if err != nil {
			t.Fatalf("failed to create service: %v", err)
		}

		ctx := context.Background()
		err = service.HealthCheck(ctx)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if mock1.healthCalls == 0 || mock2.healthCalls == 0 {
			t.Error("expected health check to be called on all clients")
		}
	})

	t.Run("some clients unhealthy", func(t *testing.T) {
		mock1 := newMockCacheRepository()
		mock2 := newMockCacheRepository()
		mock1.healthErr = errors.New("client 1 error")

		service, err := NewService(Config{
			Nodes: []string{"node1", "node2"},
			Clients: map[string]cache.CacheRepository{
				"node1": mock1,
				"node2": mock2,
			},
			Logger: slog.Default(),
		})
		if err != nil {
			t.Fatalf("failed to create service: %v", err)
		}

		ctx := context.Background()
		err = service.HealthCheck(ctx)
		if err != nil {
			t.Errorf("expected no error when some clients are healthy, got %v", err)
		}
	})

	t.Run("all clients unhealthy", func(t *testing.T) {
		mock1 := newMockCacheRepository()
		mock2 := newMockCacheRepository()
		mock1.healthErr = errors.New("client 1 error")
		mock2.healthErr = errors.New("client 2 error")

		service, err := NewService(Config{
			Nodes: []string{"node1", "node2"},
			Clients: map[string]cache.CacheRepository{
				"node1": mock1,
				"node2": mock2,
			},
			Logger: slog.Default(),
		})
		if err != nil {
			t.Fatalf("failed to create service: %v", err)
		}

		ctx := context.Background()
		err = service.HealthCheck(ctx)
		if err == nil {
			t.Error("expected error when all clients are unhealthy")
		}
	})
}

func TestService_GetNodes(t *testing.T) {
	mock1 := newMockCacheRepository()
	mock2 := newMockCacheRepository()

	service, err := NewService(Config{
		Nodes: []string{"node1", "node2"},
		Clients: map[string]cache.CacheRepository{
			"node1": mock1,
			"node2": mock2,
		},
		Logger: slog.Default(),
	})
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	nodes := service.GetNodes()
	if len(nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(nodes))
	}

	nodeMap := make(map[string]bool)
	for _, node := range nodes {
		nodeMap[node] = true
	}

	if !nodeMap["node1"] || !nodeMap["node2"] {
		t.Errorf("expected nodes node1 and node2, got %v", nodes)
	}
}

func TestService_ConsistentHashing(t *testing.T) {
	mock1 := newMockCacheRepository()
	mock2 := newMockCacheRepository()

	service, err := NewService(Config{
		Nodes: []string{"node1", "node2"},
		Clients: map[string]cache.CacheRepository{
			"node1": mock1,
			"node2": mock2,
		},
		Logger: slog.Default(),
	})
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	ctx := context.Background()

	key := "consistent-key"
	value := "consistent-value"

	err = service.Set(ctx, key, value)
	if err != nil {
		t.Fatalf("failed to set key: %v", err)
	}

	retrievedValue, err := service.Get(ctx, key)
	if err != nil {
		t.Fatalf("failed to get key: %v", err)
	}

	if retrievedValue != value {
		t.Errorf("expected %s, got %s", value, retrievedValue)
	}

	prevSet1, prevSet2 := mock1.setCalls, mock2.setCalls
	prevGet1, prevGet2 := mock1.getCalls, mock2.getCalls

	for i := 0; i < 10; i++ {
		err = service.Set(ctx, key, value)
		if err != nil {
			t.Fatalf("failed to set key on iteration %d: %v", i, err)
		}

		_, err = service.Get(ctx, key)
		if err != nil {
			t.Fatalf("failed to get key on iteration %d: %v", i, err)
		}
	}

	newSet1, newSet2 := mock1.setCalls, mock2.setCalls
	newGet1, newGet2 := mock1.getCalls, mock2.getCalls

	totalSetCalls := (newSet1 - prevSet1) + (newSet2 - prevSet2)
	totalGetCalls := (newGet1 - prevGet1) + (newGet2 - prevGet2)

	if totalSetCalls != 10 {
		t.Errorf("expected 10 total set calls, got %d", totalSetCalls)
	}

	if totalGetCalls != 10 {
		t.Errorf("expected 10 total get calls, got %d", totalGetCalls)
	}

	if (newSet1-prevSet1) == 0 && (newSet2-prevSet2) == 0 {
		t.Error("expected set calls to be routed to exactly one client")
	}

	if (newGet1-prevGet1) == 0 && (newGet2-prevGet2) == 0 {
		t.Error("expected get calls to be routed to exactly one client")
	}
}
