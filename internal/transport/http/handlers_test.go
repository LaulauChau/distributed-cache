package http

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lachau/distributed-cache/internal/domain/errors"
)

type mockCacheService struct {
	data      map[string]string
	getErr    error
	setErr    error
	deleteErr error
	healthErr error
	nodes     []string
}

func newMockCacheService() *mockCacheService {
	return &mockCacheService{
		data:  make(map[string]string),
		nodes: []string{"node1", "node2"},
	}
}

func (m *mockCacheService) Get(ctx context.Context, key string) (string, error) {
	if m.getErr != nil {
		return "", m.getErr
	}
	value, exists := m.data[key]
	if !exists {
		return "", errors.ErrKeyNotFound
	}
	return value, nil
}

func (m *mockCacheService) Set(ctx context.Context, key, value string) error {
	if m.setErr != nil {
		return m.setErr
	}
	m.data[key] = value
	return nil
}

func (m *mockCacheService) Delete(ctx context.Context, key string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	delete(m.data, key)
	return nil
}

func (m *mockCacheService) HealthCheck(ctx context.Context) error {
	return m.healthErr
}

func (m *mockCacheService) GetNodes() []string {
	return m.nodes
}

func setupHandlers() (*Handlers, *mockCacheService) {
	mockService := newMockCacheService()
	handlers := NewHandlers(Config{
		CacheService: mockService,
		Logger:       slog.Default(),
	})
	return handlers, mockService
}

func TestHandlers_GetCache(t *testing.T) {
	handlers, mockService := setupHandlers()

	tests := []struct {
		name           string
		method         string
		path           string
		setupData      func()
		setupError     func()
		expectedStatus int
		expectedBody   string
	}{
		{
			name:   "successful get",
			method: http.MethodGet,
			path:   "/cache/test-key",
			setupData: func() {
				mockService.data["test-key"] = "test-value"
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `{"key":"test-key","value":"test-value"}`,
		},
		{
			name:           "key not found",
			method:         http.MethodGet,
			path:           "/cache/nonexistent",
			setupData:      func() {},
			expectedStatus: http.StatusNotFound,
			expectedBody:   `{"error":"key not found"}`,
		},
		{
			name:           "empty key",
			method:         http.MethodGet,
			path:           "/cache/",
			setupData:      func() {},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   `{"error":"invalid key","message":"key cannot be empty"}`,
		},
		{
			name:   "service error",
			method: http.MethodGet,
			path:   "/cache/test-key",
			setupData: func() {
				mockService.data["test-key"] = "test-value"
			},
			setupError: func() {
				mockService.getErr = errors.ErrNoNodesAvailable
			},
			expectedStatus: http.StatusServiceUnavailable,
			expectedBody:   `{"error":"no nodes available"}`,
		},
		{
			name:           "wrong method",
			method:         http.MethodPost,
			path:           "/cache/test-key",
			setupData:      func() {},
			expectedStatus: http.StatusMethodNotAllowed,
			expectedBody:   `{"error":"method not allowed"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mock state
			mockService.data = make(map[string]string)
			mockService.getErr = nil

			if tt.setupData != nil {
				tt.setupData()
			}
			if tt.setupError != nil {
				tt.setupError()
			}

			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			handlers.GetCache(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			body := strings.TrimSpace(w.Body.String())
			if !strings.Contains(body, strings.TrimSpace(tt.expectedBody)) {
				t.Errorf("expected body to contain %s, got %s", tt.expectedBody, body)
			}
		})
	}
}

func TestHandlers_SetCache(t *testing.T) {
	handlers, mockService := setupHandlers()

	tests := []struct {
		name           string
		method         string
		path           string
		body           string
		setupError     func()
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "successful set",
			method:         http.MethodPut,
			path:           "/cache/test-key",
			body:           `{"value":"test-value"}`,
			expectedStatus: http.StatusCreated,
			expectedBody:   `{"key":"test-key","value":"test-value"}`,
		},
		{
			name:           "empty key",
			method:         http.MethodPut,
			path:           "/cache/",
			body:           `{"value":"test-value"}`,
			expectedStatus: http.StatusBadRequest,
			expectedBody:   `{"error":"invalid key","message":"key cannot be empty"}`,
		},
		{
			name:           "empty value",
			method:         http.MethodPut,
			path:           "/cache/test-key",
			body:           `{"value":""}`,
			expectedStatus: http.StatusBadRequest,
			expectedBody:   `{"error":"invalid value","message":"value cannot be empty"}`,
		},
		{
			name:           "invalid json",
			method:         http.MethodPut,
			path:           "/cache/test-key",
			body:           `invalid-json`,
			expectedStatus: http.StatusBadRequest,
			expectedBody:   `"error":"invalid request body"`,
		},
		{
			name:   "service error",
			method: http.MethodPut,
			path:   "/cache/test-key",
			body:   `{"value":"test-value"}`,
			setupError: func() {
				mockService.setErr = errors.ErrInvalidValue
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   `{"error":"invalid value"}`,
		},
		{
			name:           "wrong method",
			method:         http.MethodPost,
			path:           "/cache/test-key",
			body:           `{"value":"test-value"}`,
			expectedStatus: http.StatusMethodNotAllowed,
			expectedBody:   `{"error":"method not allowed"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mock state
			mockService.data = make(map[string]string)
			mockService.setErr = nil

			if tt.setupError != nil {
				tt.setupError()
			}

			req := httptest.NewRequest(tt.method, tt.path, bytes.NewBufferString(tt.body))
			w := httptest.NewRecorder()

			handlers.SetCache(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			body := strings.TrimSpace(w.Body.String())
			if !strings.Contains(body, strings.TrimSpace(tt.expectedBody)) {
				t.Errorf("expected body to contain %s, got %s", tt.expectedBody, body)
			}
		})
	}
}

func TestHandlers_DeleteCache(t *testing.T) {
	handlers, mockService := setupHandlers()

	tests := []struct {
		name           string
		method         string
		path           string
		setupData      func()
		setupError     func()
		expectedStatus int
	}{
		{
			name:   "successful delete",
			method: http.MethodDelete,
			path:   "/cache/test-key",
			setupData: func() {
				mockService.data["test-key"] = "test-value"
			},
			expectedStatus: http.StatusNoContent,
		},
		{
			name:           "empty key",
			method:         http.MethodDelete,
			path:           "/cache/",
			setupData:      func() {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:   "service error",
			method: http.MethodDelete,
			path:   "/cache/test-key",
			setupData: func() {
				mockService.data["test-key"] = "test-value"
			},
			setupError: func() {
				mockService.deleteErr = errors.ErrClientNotFound
			},
			expectedStatus: http.StatusServiceUnavailable,
		},
		{
			name:           "wrong method",
			method:         http.MethodPost,
			path:           "/cache/test-key",
			setupData:      func() {},
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mock state
			mockService.data = make(map[string]string)
			mockService.deleteErr = nil

			if tt.setupData != nil {
				tt.setupData()
			}
			if tt.setupError != nil {
				tt.setupError()
			}

			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			handlers.DeleteCache(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestHandlers_HealthCheck(t *testing.T) {
	handlers, mockService := setupHandlers()

	tests := []struct {
		name           string
		method         string
		setupError     func()
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "healthy",
			method:         http.MethodGet,
			expectedStatus: http.StatusOK,
			expectedBody:   `"status":"healthy"`,
		},
		{
			name:   "unhealthy",
			method: http.MethodGet,
			setupError: func() {
				mockService.healthErr = errors.ErrCacheUnavailable
			},
			expectedStatus: http.StatusServiceUnavailable,
			expectedBody:   `"status":"unhealthy"`,
		},
		{
			name:           "wrong method",
			method:         http.MethodPost,
			expectedStatus: http.StatusMethodNotAllowed,
			expectedBody:   `{"error":"method not allowed"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mock state
			mockService.healthErr = nil

			if tt.setupError != nil {
				tt.setupError()
			}

			req := httptest.NewRequest(tt.method, "/health", nil)
			w := httptest.NewRecorder()

			handlers.HealthCheck(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			body := strings.TrimSpace(w.Body.String())
			if !strings.Contains(body, tt.expectedBody) {
				t.Errorf("expected body to contain %s, got %s", tt.expectedBody, body)
			}
		})
	}
}

func TestHandlers_extractKey(t *testing.T) {
	handlers, _ := setupHandlers()

	tests := []struct {
		path        string
		expectedKey string
	}{
		{"/cache/test-key", "test-key"},
		{"/cache/my-key", "my-key"},
		{"/cache/", ""},
		{"/cache", ""},
		{"/other/test-key", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			key := handlers.extractKey(tt.path)
			if key != tt.expectedKey {
				t.Errorf("expected key %s, got %s", tt.expectedKey, key)
			}
		})
	}
}

func TestCacheHandler_Integration(t *testing.T) {
	handlers, _ := setupHandlers()

	// Test full workflow: set -> get -> delete
	key := "integration-test"
	value := "integration-value"

	// Set
	setBody := map[string]string{"value": value}
	setBodyBytes, _ := json.Marshal(setBody)
	req := httptest.NewRequest(http.MethodPut, "/cache/"+key, bytes.NewBuffer(setBodyBytes))
	w := httptest.NewRecorder()
	handlers.cacheHandler(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected set to return 201, got %d", w.Code)
	}

	// Get
	req = httptest.NewRequest(http.MethodGet, "/cache/"+key, nil)
	w = httptest.NewRecorder()
	handlers.cacheHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected get to return 200, got %d", w.Code)
	}

	var response CacheResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Key != key || response.Value != value {
		t.Errorf("expected key=%s value=%s, got key=%s value=%s", key, value, response.Key, response.Value)
	}

	// Delete
	req = httptest.NewRequest(http.MethodDelete, "/cache/"+key, nil)
	w = httptest.NewRecorder()
	handlers.cacheHandler(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected delete to return 204, got %d", w.Code)
	}

	// Verify deletion
	req = httptest.NewRequest(http.MethodGet, "/cache/"+key, nil)
	w = httptest.NewRecorder()
	handlers.cacheHandler(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected get after delete to return 404, got %d", w.Code)
	}
}
