package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"

	"nucc.com/mcp_srv_mgr/internal/config"
	"nucc.com/mcp_srv_mgr/pkg/types"
)

// Mock service manager for testing
type MockServiceManager struct {
	services map[string]types.ServiceInfo
	errors   map[string]error
}

func NewMockServiceManager() *MockServiceManager {
	return &MockServiceManager{
		services: make(map[string]types.ServiceInfo),
		errors:   make(map[string]error),
	}
}

func (m *MockServiceManager) AddService(name string, info types.ServiceInfo) {
	m.services[name] = info
}

func (m *MockServiceManager) SetError(operation string, err error) {
	m.errors[operation] = err
}

func (m *MockServiceManager) Start(serviceName string) error {
	if err, exists := m.errors["start"]; exists {
		return err
	}
	if service, exists := m.services[serviceName]; exists {
		service.Status = types.StatusActive
		m.services[serviceName] = service
	}
	return nil
}

func (m *MockServiceManager) Stop(serviceName string) error {
	if err, exists := m.errors["stop"]; exists {
		return err
	}
	if service, exists := m.services[serviceName]; exists {
		service.Status = types.StatusInactive
		m.services[serviceName] = service
	}
	return nil
}

func (m *MockServiceManager) Restart(serviceName string) error {
	if err, exists := m.errors["restart"]; exists {
		return err
	}
	if service, exists := m.services[serviceName]; exists {
		service.Status = types.StatusActive
		m.services[serviceName] = service
	}
	return nil
}

func (m *MockServiceManager) Enable(serviceName string) error {
	if err, exists := m.errors["enable"]; exists {
		return err
	}
	return nil
}

func (m *MockServiceManager) Disable(serviceName string) error {
	if err, exists := m.errors["disable"]; exists {
		return err
	}
	return nil
}

func (m *MockServiceManager) GetStatus(serviceName string) (types.ServiceInfo, error) {
	if err, exists := m.errors["get_status"]; exists {
		return types.ServiceInfo{}, err
	}
	if service, exists := m.services[serviceName]; exists {
		return service, nil
	}
	return types.ServiceInfo{}, fmt.Errorf("service %s not found", serviceName)
}

func (m *MockServiceManager) ListServices() ([]types.ServiceInfo, error) {
	if err, exists := m.errors["list_services"]; exists {
		return nil, err
	}
	var services []types.ServiceInfo
	for _, service := range m.services {
		services = append(services, service)
	}
	return services, nil
}

// Helper function to create test server
func createTestServer() *HTTPServer {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "127.0.0.1",
			Port: 8080,
		},
		Log: config.LogConfig{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		},
	}
	
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Reduce log noise in tests
	
	server := &HTTPServer{
		managers: make(map[types.ServiceType]types.ServiceManager),
		config:   cfg,
		logger:   logger,
	}
	
	// Add mock managers
	mockManager := NewMockServiceManager()
	server.managers[types.ServiceTypeSystemd] = mockManager
	
	// Add some test services
	mockManager.AddService("nginx", types.ServiceInfo{
		Name:        "nginx",
		Type:        types.ServiceTypeSystemd,
		Status:      types.StatusActive,
		Description: "Web server",
		PID:         1234,
		Uptime:      2 * time.Hour,
		LastChanged: time.Now().Add(-2 * time.Hour),
	})
	
	mockManager.AddService("mysql", types.ServiceInfo{
		Name:        "mysql",
		Type:        types.ServiceTypeSystemd,
		Status:      types.StatusInactive,
		Description: "Database server",
		PID:         0,
		Uptime:      0,
		LastChanged: time.Now().Add(-1 * time.Hour),
	})
	
	return server
}

func TestNewHTTPServer(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{Host: "127.0.0.1", Port: 8080},
		Log:    config.LogConfig{Level: "info", Format: "json", Output: "stdout"},
	}
	logger := logrus.New()
	
	server := NewHTTPServer(cfg, logger)
	if server == nil {
		t.Fatal("Expected HTTPServer instance, got nil")
	}
	
	if server.config != cfg {
		t.Error("Config not set correctly")
	}
	
	if server.logger != logger {
		t.Error("Logger not set correctly")
	}
}

func TestHTTPServer_SetupRoutes(t *testing.T) {
	server := createTestServer()
	router := server.SetupRoutes()
	
	if router == nil {
		t.Fatal("Expected router, got nil")
	}
	
	// Test route setup by making requests
	testRoutes := []struct {
		method string
		path   string
	}{
		{"GET", "/health"},
		{"GET", "/info"},
		{"GET", "/services"},
		{"GET", "/services/nginx/status"},
	}
	
	for _, route := range testRoutes {
		req := httptest.NewRequest(route.method, route.path, nil)
		w := httptest.NewRecorder()
		
		router.ServeHTTP(w, req)
		
		// Should not return 404 for valid routes
		if w.Code == 404 {
			t.Errorf("Route %s %s returned 404", route.method, route.path)
		}
	}
}

func TestHTTPServer_HandleHealth(t *testing.T) {
	server := createTestServer()
	router := server.SetupRoutes()
	
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	
	router.ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	
	if response["status"] != "healthy" {
		t.Errorf("Expected status 'healthy', got %v", response["status"])
	}
}

func TestHTTPServer_HandleInfo(t *testing.T) {
	server := createTestServer()
	router := server.SetupRoutes()
	
	req := httptest.NewRequest("GET", "/info", nil)
	w := httptest.NewRecorder()
	
	router.ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	
	if response["name"] != "MCP Service Manager" {
		t.Errorf("Expected name 'MCP Service Manager', got %v", response["name"])
	}
}

func TestHTTPServer_HandleListServices(t *testing.T) {
	server := createTestServer()
	router := server.SetupRoutes()
	
	req := httptest.NewRequest("GET", "/services", nil)
	w := httptest.NewRecorder()
	
	router.ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	var response types.ServiceListResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	
	if !response.Success {
		t.Error("Expected success to be true")
	}
	
	if len(response.Services) != 2 {
		t.Errorf("Expected 2 services, got %d", len(response.Services))
	}
}

func TestHTTPServer_HandleListServices_WithType(t *testing.T) {
	server := createTestServer()
	router := server.SetupRoutes()
	
	req := httptest.NewRequest("GET", "/services?type=systemd", nil)
	w := httptest.NewRecorder()
	
	router.ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	var response types.ServiceListResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	
	if !response.Success {
		t.Error("Expected success to be true")
	}
	
	for _, service := range response.Services {
		if service.Type != types.ServiceTypeSystemd {
			t.Errorf("Expected all services to be systemd type, got %s", service.Type)
		}
	}
}

func TestHTTPServer_HandleGetStatus(t *testing.T) {
	server := createTestServer()
	router := server.SetupRoutes()
	
	req := httptest.NewRequest("GET", "/services/nginx/status", nil)
	w := httptest.NewRecorder()
	
	router.ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	var response types.ServiceResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	
	if !response.Success {
		t.Error("Expected success to be true")
	}
	
	if response.Service.Name != "nginx" {
		t.Errorf("Expected service name 'nginx', got %s", response.Service.Name)
	}
}

func TestHTTPServer_HandleStartService(t *testing.T) {
	server := createTestServer()
	router := server.SetupRoutes()
	
	req := httptest.NewRequest("POST", "/services/mysql/start", nil)
	w := httptest.NewRecorder()
	
	router.ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	var response types.ServiceResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	
	if !response.Success {
		t.Error("Expected success to be true")
	}
	
	if !strings.Contains(response.Message, "started successfully") {
		t.Errorf("Expected success message, got %s", response.Message)
	}
}

func TestHTTPServer_HandleServiceAction(t *testing.T) {
	server := createTestServer()
	router := server.SetupRoutes()
	
	requestBody := types.ServiceRequest{
		Name:   "nginx",
		Type:   types.ServiceTypeSystemd,
		Action: "stop",
	}
	
	bodyBytes, _ := json.Marshal(requestBody)
	req := httptest.NewRequest("POST", "/services/action", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	
	router.ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	var response types.ServiceResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	
	if !response.Success {
		t.Error("Expected success to be true")
	}
}

func TestHTTPServer_HandleNonexistentService(t *testing.T) {
	server := createTestServer()
	router := server.SetupRoutes()
	
	req := httptest.NewRequest("GET", "/services/nonexistent/status", nil)
	w := httptest.NewRecorder()
	
	router.ServeHTTP(w, req)
	
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHTTPServer_HandleUnsupportedServiceType(t *testing.T) {
	server := createTestServer()
	router := server.SetupRoutes()
	
	req := httptest.NewRequest("GET", "/services?type=unsupported", nil)
	w := httptest.NewRecorder()
	
	router.ServeHTTP(w, req)
	
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHTTPServer_HandleInvalidJSON(t *testing.T) {
	server := createTestServer()
	router := server.SetupRoutes()
	
	req := httptest.NewRequest("POST", "/services/action", strings.NewReader("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	
	router.ServeHTTP(w, req)
	
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHTTPServer_GetServiceManager(t *testing.T) {
	server := createTestServer()
	
	// Test with specific service type
	manager, err := server.getServiceManager("nginx", "systemd")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if manager == nil {
		t.Error("Expected manager, got nil")
	}
	
	// Test with auto-detection
	manager, err = server.getServiceManager("nginx", "")
	if err != nil {
		t.Errorf("Expected no error for auto-detection, got %v", err)
	}
	if manager == nil {
		t.Error("Expected manager, got nil")
	}
	
	// Test with unsupported type
	_, err = server.getServiceManager("nginx", "unsupported")
	if err == nil {
		t.Error("Expected error for unsupported type, got none")
	}
	
	// Test with nonexistent service
	_, err = server.getServiceManager("nonexistent", "")
	if err == nil {
		t.Error("Expected error for nonexistent service, got none")
	}
}

func TestHTTPServer_GetAvailableManagers(t *testing.T) {
	server := createTestServer()
	managers := server.getAvailableManagers()
	
	if len(managers) == 0 {
		t.Error("Expected at least one manager")
	}
	
	found := false
	for _, manager := range managers {
		if manager == "systemd" {
			found = true
			break
		}
	}
	
	if !found {
		t.Error("Expected systemd manager in list")
	}
}

// Benchmark tests
func BenchmarkHTTPServer_HandleListServices(b *testing.B) {
	server := createTestServer()
	router := server.SetupRoutes()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/services", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}

func BenchmarkHTTPServer_HandleGetStatus(b *testing.B) {
	server := createTestServer()
	router := server.SetupRoutes()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/services/nginx/status", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}