// +build integration

package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sirupsen/logrus"

	"nucc.com/mcp_srv_mgr/internal/config"
	"nucc.com/mcp_srv_mgr/internal/server"
	"nucc.com/mcp_srv_mgr/pkg/types"
)

// Integration tests that require actual system resources
// Run with: go test -tags=integration

func TestIntegration_FullHTTPWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// 创建真实的服务器配置
	cfg := &config.Config{
		Server: config.ServerConfig{Host: "127.0.0.1", Port: 0}, // 使用随机端口
		Log:    config.LogConfig{Level: "error", Format: "json", Output: "stdout"},
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	httpServer := server.NewHTTPServer(cfg, logger)
	router := httpServer.SetupRoutes()

	// 创建测试服务器
	testServer := httptest.NewServer(router)
	defer testServer.Close()

	client := &http.Client{Timeout: 10 * time.Second}

	t.Run("Health Check", func(t *testing.T) {
		resp, err := client.Get(testServer.URL + "/health")
		if err != nil {
			t.Fatalf("Health check failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		var healthResponse map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&healthResponse); err != nil {
			t.Fatalf("Failed to decode health response: %v", err)
		}

		if healthResponse["status"] != "healthy" {
			t.Errorf("Expected healthy status, got %v", healthResponse["status"])
		}
	})

	t.Run("Server Info", func(t *testing.T) {
		resp, err := client.Get(testServer.URL + "/info")
		if err != nil {
			t.Fatalf("Info request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		var infoResponse map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&infoResponse); err != nil {
			t.Fatalf("Failed to decode info response: %v", err)
		}

		if infoResponse["name"] != "MCP Service Manager" {
			t.Errorf("Expected name 'MCP Service Manager', got %v", infoResponse["name"])
		}
	})

	t.Run("List Services", func(t *testing.T) {
		resp, err := client.Get(testServer.URL + "/services")
		if err != nil {
			t.Fatalf("List services failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		var servicesResponse types.ServiceListResponse
		if err := json.NewDecoder(resp.Body).Decode(&servicesResponse); err != nil {
			t.Fatalf("Failed to decode services response: %v", err)
		}

		if !servicesResponse.Success {
			t.Error("Expected success to be true")
		}

		t.Logf("Found %d services in total", len(servicesResponse.Services))

		// 验证服务数据结构
		for i, service := range servicesResponse.Services {
			if service.Name == "" {
				t.Errorf("Service %d has empty name", i)
			}
			if service.Type == "" {
				t.Errorf("Service %d has empty type", i)
			}
			
			// 限制检查前5个服务
			if i >= 5 {
				break
			}
		}
	})

	t.Run("Service Action - Generic Endpoint", func(t *testing.T) {
		// 获取可用服务
		resp, err := client.Get(testServer.URL + "/services")
		if err != nil {
			t.Fatalf("Failed to get services: %v", err)
		}
		defer resp.Body.Close()

		var servicesResponse types.ServiceListResponse
		if err := json.NewDecoder(resp.Body).Decode(&servicesResponse); err != nil {
			t.Fatalf("Failed to decode services: %v", err)
		}

		if len(servicesResponse.Services) == 0 {
			t.Skip("No services available for testing")
		}

		// 选择第一个服务进行测试
		testService := servicesResponse.Services[0]
		
		// 测试获取服务状态
		actionRequest := types.ServiceRequest{
			Name:   testService.Name,
			Type:   testService.Type,
			Action: "status",
		}

		actionBody, _ := json.Marshal(actionRequest)
		resp, err = client.Post(testServer.URL+"/services/action", "application/json", bytes.NewReader(actionBody))
		
		// 注意：这个操作可能会失败，因为我们可能没有权限
		// 但应该返回一个结构化的响应，而不是崩溃
		if err != nil {
			t.Logf("Service action failed (expected): %v", err)
		} else {
			defer resp.Body.Close()
			t.Logf("Service action response status: %d", resp.StatusCode)
		}
	})
}

func TestIntegration_ServiceManagersAvailability(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := &config.Config{
		Server: config.ServerConfig{Host: "127.0.0.1", Port: 0},
		Log:    config.LogConfig{Level: "error", Format: "json", Output: "stdout"},
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	httpServer := server.NewHTTPServer(cfg, logger)
	router := httpServer.SetupRoutes()

	testServer := httptest.NewServer(router)
	defer testServer.Close()

	client := &http.Client{Timeout: 10 * time.Second}

	// 检查不同类型的服务管理器
	serviceTypes := []string{"systemd", "sysv", "docker"}

	for _, serviceType := range serviceTypes {
		t.Run(fmt.Sprintf("Manager_%s", serviceType), func(t *testing.T) {
			url := fmt.Sprintf("%s/services?type=%s", testServer.URL, serviceType)
			resp, err := client.Get(url)
			if err != nil {
				t.Fatalf("Failed to get %s services: %v", serviceType, err)
			}
			defer resp.Body.Close()

			var servicesResponse types.ServiceListResponse
			if err := json.NewDecoder(resp.Body).Decode(&servicesResponse); err != nil {
				t.Fatalf("Failed to decode %s services response: %v", serviceType, err)
			}

			// 如果管理器不可用，应该返回错误或空列表
			if resp.StatusCode == http.StatusBadRequest {
				t.Logf("%s manager not available (expected)", serviceType)
			} else if resp.StatusCode == http.StatusOK {
				t.Logf("%s manager available, found %d services", serviceType, len(servicesResponse.Services))
			} else {
				t.Errorf("Unexpected status code for %s: %d", serviceType, resp.StatusCode)
			}
		})
	}
}

func TestIntegration_ErrorHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := &config.Config{
		Server: config.ServerConfig{Host: "127.0.0.1", Port: 0},
		Log:    config.LogConfig{Level: "error", Format: "json", Output: "stdout"},
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	httpServer := server.NewHTTPServer(cfg, logger)
	router := httpServer.SetupRoutes()

	testServer := httptest.NewServer(router)
	defer testServer.Close()

	client := &http.Client{Timeout: 10 * time.Second}

	t.Run("Nonexistent Service", func(t *testing.T) {
		resp, err := client.Get(testServer.URL + "/services/nonexistent-service-12345/status")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		// 应该返回错误状态
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected status 400 for nonexistent service, got %d", resp.StatusCode)
		}
	})

	t.Run("Invalid Service Type", func(t *testing.T) {
		resp, err := client.Get(testServer.URL + "/services?type=invalid-type")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		// 应该返回错误状态
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected status 400 for invalid service type, got %d", resp.StatusCode)
		}
	})

	t.Run("Invalid JSON Request", func(t *testing.T) {
		invalidJSON := bytes.NewReader([]byte("{invalid json"))
		resp, err := client.Post(testServer.URL+"/services/action", "application/json", invalidJSON)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		// 应该返回错误状态
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected status 400 for invalid JSON, got %d", resp.StatusCode)
		}
	})

	t.Run("Missing Required Fields", func(t *testing.T) {
		incompleteRequest := map[string]interface{}{
			"name": "test-service",
			// missing action and type
		}
		
		requestBody, _ := json.Marshal(incompleteRequest)
		resp, err := client.Post(testServer.URL+"/services/action", "application/json", bytes.NewReader(requestBody))
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		// 应该处理缺少字段的情况
		t.Logf("Response status for incomplete request: %d", resp.StatusCode)
	})
}

func TestIntegration_Concurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := &config.Config{
		Server: config.ServerConfig{Host: "127.0.0.1", Port: 0},
		Log:    config.LogConfig{Level: "error", Format: "json", Output: "stdout"},
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	httpServer := server.NewHTTPServer(cfg, logger)
	router := httpServer.SetupRoutes()

	testServer := httptest.NewServer(router)
	defer testServer.Close()

	client := &http.Client{Timeout: 10 * time.Second}

	// 并发测试
	const numRequests = 10
	results := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		go func(id int) {
			resp, err := client.Get(testServer.URL + "/services")
			if err != nil {
				results <- fmt.Errorf("request %d failed: %v", id, err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				results <- fmt.Errorf("request %d got status %d", id, resp.StatusCode)
				return
			}

			results <- nil
		}(i)
	}

	// 收集结果
	var errors []error
	for i := 0; i < numRequests; i++ {
		if err := <-results; err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		t.Errorf("Concurrent requests failed: %v", errors)
	} else {
		t.Logf("All %d concurrent requests succeeded", numRequests)
	}
}

// Benchmark integration tests
func BenchmarkIntegration_ListServices(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping integration benchmark in short mode")
	}

	cfg := &config.Config{
		Server: config.ServerConfig{Host: "127.0.0.1", Port: 0},
		Log:    config.LogConfig{Level: "error", Format: "json", Output: "stdout"},
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	httpServer := server.NewHTTPServer(cfg, logger)
	router := httpServer.SetupRoutes()

	testServer := httptest.NewServer(router)
	defer testServer.Close()

	client := &http.Client{Timeout: 10 * time.Second}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := client.Get(testServer.URL + "/services")
		if err != nil {
			b.Fatalf("Request failed: %v", err)
		}
		resp.Body.Close()
	}
}