package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHTTPREST(t *testing.T) {
	serverBinary := filepath.Join("..", "mcp-server")
	
	if _, err := os.Stat(serverBinary); os.IsNotExist(err) {
		t.Fatalf("Server binary not found at %s. Please run 'go build -o mcp-server cmd/server/main.go' first", serverBinary)
	}

	helper := NewTestHelper(serverBinary)
	defer helper.StopAllServers()

	const httpPort = 8081

	t.Run("StartHTTPServer", func(t *testing.T) {
		server, err := helper.StartServer("http", httpPort)
		if err != nil {
			t.Fatalf("Failed to start HTTP server: %v", err)
		}

		if server.mode != "http" {
			t.Errorf("Expected mode 'http', got '%s'", server.mode)
		}

		if server.port != httpPort {
			t.Errorf("Expected port %d, got %d", httpPort, server.port)
		}

		// 验证服务器已启动
		if !server.started {
			t.Error("Server should be marked as started")
		}
	})

	t.Run("HealthCheck", func(t *testing.T) {
		_, err := helper.StartServer("http", httpPort)
		if err != nil {
			t.Fatalf("Failed to start HTTP server: %v", err)
		}

		client := NewHTTPClient(fmt.Sprintf("http://127.0.0.1:%d", httpPort))
		
		resp, err := client.Get("/health")
		if err != nil {
			t.Fatalf("Failed to get health endpoint: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read response body: %v", err)
		}

		var healthResponse map[string]interface{}
		err = json.Unmarshal(body, &healthResponse)
		if err != nil {
			t.Fatalf("Failed to unmarshal health response: %v", err)
		}

		if healthResponse["status"] != "healthy" {
			t.Errorf("Expected status 'healthy', got '%v'", healthResponse["status"])
		}

		// 验证包含预期字段
		expectedFields := []string{"status", "timestamp", "managers"}
		for _, field := range expectedFields {
			if _, exists := healthResponse[field]; !exists {
				t.Errorf("Expected field '%s' not found in health response", field)
			}
		}
	})

	t.Run("ListServices", func(t *testing.T) {
		_, err := helper.StartServer("http", httpPort)
		if err != nil {
			t.Fatalf("Failed to start HTTP server: %v", err)
		}

		client := NewHTTPClient(fmt.Sprintf("http://127.0.0.1:%d", httpPort))
		
		resp, err := client.Get("/services")
		if err != nil {
			t.Fatalf("Failed to get services endpoint: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read response body: %v", err)
		}

		var servicesResponse map[string]interface{}
		err = json.Unmarshal(body, &servicesResponse)
		if err != nil {
			t.Fatalf("Failed to unmarshal services response: %v", err)
		}

		// 验证响应结构
		if servicesResponse["success"] != true {
			t.Errorf("Expected success true, got %v", servicesResponse["success"])
		}

		if servicesResponse["message"] == nil {
			t.Error("Expected message field in response")
		}

		services, ok := servicesResponse["services"].([]interface{})
		if !ok {
			t.Fatalf("Expected services to be an array, got %T", servicesResponse["services"])
		}

		t.Logf("Found %d services", len(services))

		// 如果有服务，验证第一个服务的结构
		if len(services) > 0 {
			firstService, ok := services[0].(map[string]interface{})
			if !ok {
				t.Errorf("Expected service to be an object, got %T", services[0])
			} else {
				expectedServiceFields := []string{"name", "type", "status"}
				for _, field := range expectedServiceFields {
					if _, exists := firstService[field]; !exists {
						t.Errorf("Expected field '%s' not found in service", field)
					}
				}
			}
		}
	})

	t.Run("ListServicesWithFilter", func(t *testing.T) {
		_, err := helper.StartServer("http", httpPort)
		if err != nil {
			t.Fatalf("Failed to start HTTP server: %v", err)
		}

		client := NewHTTPClient(fmt.Sprintf("http://127.0.0.1:%d", httpPort))
		
		// 测试不同的过滤器
		filters := []string{"systemd", "docker", "sysv"}
		
		for _, filter := range filters {
			t.Run(fmt.Sprintf("Filter_%s", filter), func(t *testing.T) {
				resp, err := client.Get(fmt.Sprintf("/services?type=%s", filter))
				if err != nil {
					t.Fatalf("Failed to get services with filter %s: %v", filter, err)
				}
				defer resp.Body.Close()

				if resp.StatusCode != http.StatusOK {
					t.Errorf("Expected status 200 for filter %s, got %d", filter, resp.StatusCode)
				}

				body, err := io.ReadAll(resp.Body)
				if err != nil {
					t.Fatalf("Failed to read response body for filter %s: %v", filter, err)
				}

				var servicesResponse map[string]interface{}
				err = json.Unmarshal(body, &servicesResponse)
				if err != nil {
					t.Fatalf("Failed to unmarshal services response for filter %s: %v", filter, err)
				}

				if servicesResponse["success"] != true {
					t.Errorf("Expected success true for filter %s, got %v", filter, servicesResponse["success"])
				}

				services, ok := servicesResponse["services"].([]interface{})
				if !ok {
					t.Fatalf("Expected services to be an array for filter %s, got %T", filter, servicesResponse["services"])
				}

				// 验证所有返回的服务都是指定类型的
				for i, service := range services {
					serviceObj, ok := service.(map[string]interface{})
					if !ok {
						t.Errorf("Service %d should be an object for filter %s", i, filter)
						continue
					}
					
					serviceType, ok := serviceObj["type"].(string)
					if !ok {
						t.Errorf("Service %d type should be a string for filter %s", i, filter)
						continue
					}
					
					if serviceType != filter {
						t.Errorf("Expected service type %s, got %s (service %d)", filter, serviceType, i)
					}
				}

				t.Logf("Filter %s returned %d services", filter, len(services))
			})
		}
	})

	t.Run("GetServiceStatus", func(t *testing.T) {
		_, err := helper.StartServer("http", httpPort)
		if err != nil {
			t.Fatalf("Failed to start HTTP server: %v", err)
		}

		client := NewHTTPClient(fmt.Sprintf("http://127.0.0.1:%d", httpPort))
		
		// 先获取服务列表，找一个存在的服务
		resp, err := client.Get("/services")
		if err != nil {
			t.Fatalf("Failed to get services: %v", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read services response: %v", err)
		}

		var servicesResponse map[string]interface{}
		json.Unmarshal(body, &servicesResponse)
		
		services, ok := servicesResponse["services"].([]interface{})
		if !ok || len(services) == 0 {
			t.Skip("No services found, skipping service status test")
		}

		// 使用第一个服务进行测试
		firstService := services[0].(map[string]interface{})
		serviceName := firstService["name"].(string)

		// 获取服务状态
		statusResp, err := client.Get(fmt.Sprintf("/services/%s/status", serviceName))
		if err != nil {
			t.Fatalf("Failed to get service status: %v", err)
		}
		defer statusResp.Body.Close()

		// 应该返回200或404（如果服务不存在）
		if statusResp.StatusCode != http.StatusOK && statusResp.StatusCode != http.StatusNotFound {
			t.Errorf("Expected status 200 or 404, got %d", statusResp.StatusCode)
		}

		if statusResp.StatusCode == http.StatusOK {
			body, err := io.ReadAll(statusResp.Body)
			if err != nil {
				t.Fatalf("Failed to read status response: %v", err)
			}

			var statusResponse map[string]interface{}
			err = json.Unmarshal(body, &statusResponse)
			if err != nil {
				t.Fatalf("Failed to unmarshal status response: %v", err)
			}

			if statusResponse["success"] != true {
				t.Errorf("Expected success true, got %v", statusResponse["success"])
			}

			service, ok := statusResponse["service"].(map[string]interface{})
			if !ok {
				t.Fatalf("Expected service to be an object, got %T", statusResponse["service"])
			}

			if service["name"] != serviceName {
				t.Errorf("Expected service name %s, got %v", serviceName, service["name"])
			}

			expectedFields := []string{"name", "type", "status"}
			for _, field := range expectedFields {
				if _, exists := service[field]; !exists {
					t.Errorf("Expected field '%s' not found in service status", field)
				}
			}

			t.Logf("Service %s status: %v", serviceName, service["status"])
		}
	})

	t.Run("ServiceActions", func(t *testing.T) {
		_, err := helper.StartServer("http", httpPort)
		if err != nil {
			t.Fatalf("Failed to start HTTP server: %v", err)
		}

		client := NewHTTPClient(fmt.Sprintf("http://127.0.0.1:%d", httpPort))
		
		// 测试通用服务操作端点
		actionRequest := map[string]interface{}{
			"name":   "test-service",
			"type":   "systemd",
			"action": "status", // 使用状态查询，这个操作相对安全
		}

		jsonData, _ := json.Marshal(actionRequest)
		
		resp, err := client.Post("/services/action", bytes.NewBuffer(jsonData))
		if err != nil {
			t.Fatalf("Failed to post service action: %v", err)
		}
		defer resp.Body.Close()

		// 这里我们期望404或其他错误，因为test-service可能不存在
		// 但不应该是500内部服务器错误
		if resp.StatusCode == http.StatusInternalServerError {
			body, _ := io.ReadAll(resp.Body)
			t.Errorf("Unexpected internal server error: %s", string(body))
		}

		// 读取响应内容
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read response body: %v", err)
		}

		t.Logf("Service action response (status %d): %s", resp.StatusCode, string(body))
	})

	t.Run("InvalidEndpoint", func(t *testing.T) {
		_, err := helper.StartServer("http", httpPort)
		if err != nil {
			t.Fatalf("Failed to start HTTP server: %v", err)
		}

		client := NewHTTPClient(fmt.Sprintf("http://127.0.0.1:%d", httpPort))
		
		resp, err := client.Get("/invalid/endpoint")
		if err != nil {
			t.Fatalf("Failed to get invalid endpoint: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Expected status 404 for invalid endpoint, got %d", resp.StatusCode)
		}
	})

	t.Run("CORS", func(t *testing.T) {
		_, err := helper.StartServer("http", httpPort)
		if err != nil {
			t.Fatalf("Failed to start HTTP server: %v", err)
		}

		// 创建OPTIONS请求来测试CORS
		req, err := http.NewRequest("OPTIONS", fmt.Sprintf("http://127.0.0.1:%d/services", httpPort), nil)
		if err != nil {
			t.Fatalf("Failed to create OPTIONS request: %v", err)
		}

		req.Header.Set("Origin", "http://example.com")
		req.Header.Set("Access-Control-Request-Method", "GET")

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Failed to send OPTIONS request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200 for OPTIONS request, got %d", resp.StatusCode)
		}

		// 验证CORS头
		corsHeaders := map[string]string{
			"Access-Control-Allow-Origin":  "*",
			"Access-Control-Allow-Methods": "GET, POST, PUT, DELETE, OPTIONS",
		}

		for header, expectedValue := range corsHeaders {
			actualValue := resp.Header.Get(header)
			if actualValue != expectedValue {
				t.Errorf("Expected %s header '%s', got '%s'", header, expectedValue, actualValue)
			}
		}
	})
}

func TestHTTPRESTWithLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	serverBinary := filepath.Join("..", "mcp-server")
	helper := NewTestHelper(serverBinary)
	defer helper.StopAllServers()

	const httpPort = 8082

	server, err := helper.StartServer("http", httpPort)
	if err != nil {
		t.Fatalf("Failed to start HTTP server: %v", err)
	}

	// 在后台读取日志
	go LogOutput("HTTP-STDERR", server.GetStderr())

	client := NewHTTPClient(fmt.Sprintf("http://127.0.0.1:%d", httpPort))

	// 并发请求测试
	const numRequests = 20
	const numWorkers = 5

	requestChan := make(chan int, numRequests)
	resultChan := make(chan error, numRequests)

	// 填充请求队列
	for i := 0; i < numRequests; i++ {
		requestChan <- i
	}
	close(requestChan)

	// 启动工作协程
	for w := 0; w < numWorkers; w++ {
		go func(workerID int) {
			for requestID := range requestChan {
				t.Logf("Worker %d processing request %d", workerID, requestID)

				// 交替进行不同的请求
				var err error
				switch requestID % 3 {
				case 0:
					_, err = client.Get("/health")
				case 1:
					_, err = client.Get("/services")
				case 2:
					_, err = client.Get("/info")
				}

				if err != nil {
					resultChan <- fmt.Errorf("worker %d, request %d failed: %v", workerID, requestID, err)
				} else {
					resultChan <- nil
				}
			}
		}(w)
	}

	// 等待所有请求完成
	var errors []error
	for i := 0; i < numRequests; i++ {
		if err := <-resultChan; err != nil {
			errors = append(errors, err)
		}
	}

	// 检查错误
	if len(errors) > 0 {
		t.Errorf("Load test failed with %d errors:", len(errors))
		for i, err := range errors {
			t.Errorf("  Error %d: %v", i+1, err)
			if i >= 4 { // 只显示前5个错误
				t.Errorf("  ... and %d more errors", len(errors)-5)
				break
			}
		}
	} else {
		t.Logf("Load test completed successfully with %d requests", numRequests)
	}
}