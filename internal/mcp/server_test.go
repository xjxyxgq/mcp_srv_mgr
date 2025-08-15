package mcp

import (
	"strings"
	"testing"

	"github.com/sirupsen/logrus"

	"nucc.com/mcp_srv_mgr/pkg/types"
)

func TestNewServer(t *testing.T) {
	logger := logrus.New()
	server := NewServer(logger)
	
	if server == nil {
		t.Fatal("Expected Server instance, got nil")
	}
	
	if server.logger != logger {
		t.Error("Logger not properly set")
	}
	
	if server.managers == nil {
		t.Error("Managers map not initialized")
	}
	
	if server.initialized {
		t.Error("Server should not be initialized initially")
	}
	
	if server.logLevel != types.LoggingLevelInfo {
		t.Errorf("Expected default log level %s, got %s", types.LoggingLevelInfo, server.logLevel)
	}
}

func TestServer_HandleInitialize(t *testing.T) {
	logger := logrus.New()
	server := NewServer(logger)
	
	request := &types.MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params: types.InitializeParams{
			ProtocolVersion: "2024-11-05",
			Capabilities:    types.ClientCapabilities{},
			ClientInfo:      types.ClientInfo{Name: "test-client", Version: "1.0"},
		},
	}
	
	response := server.handleInitialize(request)
	
	if response == nil {
		t.Fatal("Expected response, got nil")
	}
	
	if response.ID != request.ID {
		t.Errorf("Expected response ID %v, got %v", request.ID, response.ID)
	}
	
	if response.Error != nil {
		t.Errorf("Expected no error, got %v", response.Error)
	}
	
	// 验证返回的结果
	result, ok := response.Result.(types.InitializeResult)
	if !ok {
		t.Fatal("Expected InitializeResult in response")
	}
	
	if result.ProtocolVersion != "2024-11-05" {
		t.Errorf("Expected protocol version 2024-11-05, got %s", result.ProtocolVersion)
	}
	
	if result.ServerInfo.Name != "Linux Service Manager" {
		t.Errorf("Expected server name 'Linux Service Manager', got %s", result.ServerInfo.Name)
	}
}

func TestServer_HandleInitialized(t *testing.T) {
	logger := logrus.New()
	server := NewServer(logger)
	
	request := &types.MCPRequest{
		JSONRPC: "2.0",
		Method:  "initialized",
	}
	
	if server.initialized {
		t.Error("Server should not be initialized initially")
	}
	
	response := server.handleInitialized(request)
	
	// initialized是通知，不返回响应
	if response != nil {
		t.Error("Expected no response for notification, got response")
	}
	
	if !server.initialized {
		t.Error("Server should be initialized after handling initialized notification")
	}
}

func TestServer_HandleListTools(t *testing.T) {
	logger := logrus.New()
	server := NewServer(logger)
	
	request := &types.MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
	}
	
	response := server.handleListTools(request)
	
	if response == nil {
		t.Fatal("Expected response, got nil")
	}
	
	if response.Error != nil {
		t.Errorf("Expected no error, got %v", response.Error)
	}
	
	result, ok := response.Result.(types.ListToolsResult)
	if !ok {
		t.Fatal("Expected ListToolsResult in response")
	}
	
	// 验证工具列表
	expectedTools := []string{
		"list_services", "get_service_status", "start_service",
		"stop_service", "restart_service", "enable_service",
		"disable_service", "get_docker_logs",
	}
	
	if len(result.Tools) != len(expectedTools) {
		t.Errorf("Expected %d tools, got %d", len(expectedTools), len(result.Tools))
	}
	
	for i, expectedName := range expectedTools {
		if i >= len(result.Tools) {
			t.Errorf("Missing tool: %s", expectedName)
			continue
		}
		if result.Tools[i].Name != expectedName {
			t.Errorf("Expected tool %s, got %s", expectedName, result.Tools[i].Name)
		}
		if result.Tools[i].Description == "" {
			t.Errorf("Tool %s has empty description", expectedName)
		}
	}
}

func TestServer_HandleListPrompts(t *testing.T) {
	logger := logrus.New()
	server := NewServer(logger)
	
	request := &types.MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "prompts/list",
	}
	
	response := server.handleListPrompts(request)
	
	if response == nil {
		t.Fatal("Expected response, got nil")
	}
	
	if response.Error != nil {
		t.Errorf("Expected no error, got %v", response.Error)
	}
	
	result, ok := response.Result.(types.ListPromptsResult)
	if !ok {
		t.Fatal("Expected ListPromptsResult in response")
	}
	
	expectedPrompts := []string{"service_management_help", "service_troubleshooting"}
	
	if len(result.Prompts) != len(expectedPrompts) {
		t.Errorf("Expected %d prompts, got %d", len(expectedPrompts), len(result.Prompts))
	}
	
	for i, expectedName := range expectedPrompts {
		if i >= len(result.Prompts) {
			t.Errorf("Missing prompt: %s", expectedName)
			continue
		}
		if result.Prompts[i].Name != expectedName {
			t.Errorf("Expected prompt %s, got %s", expectedName, result.Prompts[i].Name)
		}
	}
}

func TestServer_HandleSetLogLevel(t *testing.T) {
	logger := logrus.New()
	server := NewServer(logger)
	
	request := &types.MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "logging/setLevel",
		Params: types.SetLevelParams{
			Level: types.LoggingLevelDebug,
		},
	}
	
	response := server.handleSetLogLevel(request)
	
	if response == nil {
		t.Fatal("Expected response, got nil")
	}
	
	if response.Error != nil {
		t.Errorf("Expected no error, got %v", response.Error)
	}
	
	if server.logLevel != types.LoggingLevelDebug {
		t.Errorf("Expected log level %s, got %s", types.LoggingLevelDebug, server.logLevel)
	}
}

func TestServer_HandleCallTool_ListServices(t *testing.T) {
	logger := logrus.New()
	server := NewServer(logger)
	
	request := &types.MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params: types.CallToolParams{
			Name:      "list_services",
			Arguments: map[string]interface{}{},
		},
	}
	
	response := server.handleCallTool(request)
	
	if response == nil {
		t.Fatal("Expected response, got nil")
	}
	
	if response.Error != nil {
		t.Errorf("Expected no error, got %v", response.Error)
	}
	
	result, ok := response.Result.(types.CallToolResult)
	if !ok {
		t.Fatal("Expected CallToolResult in response")
	}
	
	if len(result.Content) == 0 {
		t.Error("Expected content in result")
	}
	
	if result.Content[0].Type != "text" {
		t.Errorf("Expected content type 'text', got %s", result.Content[0].Type)
	}
}

func TestServer_HandleCallTool_GetServiceStatus(t *testing.T) {
	logger := logrus.New()
	server := NewServer(logger)
	
	request := &types.MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params: types.CallToolParams{
			Name: "get_service_status",
			Arguments: map[string]interface{}{
				"service_name": "nonexistent-service",
			},
		},
	}
	
	response := server.handleCallTool(request)
	
	if response == nil {
		t.Fatal("Expected response, got nil")
	}
	
	// 对于不存在的服务，应该返回错误内容而不是HTTP错误
	result, ok := response.Result.(types.CallToolResult)
	if !ok {
		t.Fatal("Expected CallToolResult in response")
	}
	
	if len(result.Content) == 0 {
		t.Error("Expected content in result")
	}
	
	// 错误响应应该在content中包含错误信息
	if !strings.Contains(result.Content[0].Text, "Error:") && !result.IsError {
		t.Error("Expected error content for nonexistent service")
	}
}

func TestServer_HandleGetPrompt_ServiceManagementHelp(t *testing.T) {
	logger := logrus.New()
	server := NewServer(logger)
	
	request := &types.MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "prompts/get",
		Params: types.GetPromptParams{
			Name:      "service_management_help",
			Arguments: map[string]interface{}{},
		},
	}
	
	response := server.handleGetPrompt(request)
	
	if response == nil {
		t.Fatal("Expected response, got nil")
	}
	
	if response.Error != nil {
		t.Errorf("Expected no error, got %v", response.Error)
	}
	
	result, ok := response.Result.(types.GetPromptResult)
	if !ok {
		t.Fatal("Expected GetPromptResult in response")
	}
	
	if len(result.Messages) == 0 {
		t.Error("Expected messages in prompt result")
	}
	
	if result.Messages[0].Role != "assistant" {
		t.Errorf("Expected role 'assistant', got %s", result.Messages[0].Role)
	}
	
	if !strings.Contains(result.Messages[0].Content, "Service Management") {
		t.Error("Expected service management content in prompt")
	}
}

func TestServer_HandleUnknownMethod(t *testing.T) {
	logger := logrus.New()
	server := NewServer(logger)
	
	request := &types.MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "unknown/method",
	}
	
	response := server.handleRequest(request)
	
	if response == nil {
		t.Fatal("Expected response, got nil")
	}
	
	if response.Error == nil {
		t.Error("Expected error for unknown method")
	}
	
	if response.Error.Code != types.MethodNotFound {
		t.Errorf("Expected error code %d, got %d", types.MethodNotFound, response.Error.Code)
	}
}

func TestServer_FormatServicesOutput(t *testing.T) {
	logger := logrus.New()
	server := NewServer(logger)
	
	services := []types.ServiceInfo{
		{
			Name:        "nginx",
			Type:        types.ServiceTypeSystemd,
			Status:      types.StatusActive,
			Description: "Web server",
		},
		{
			Name:        "mysql",
			Type:        types.ServiceTypeSystemd,
			Status:      types.StatusInactive,
			Description: "Database server",
		},
		{
			Name:        "web-app",
			Type:        types.ServiceTypeDocker,
			Status:      types.StatusActive,
			Description: "Web application",
		},
	}
	
	output := server.formatServicesOutput(services)
	
	if !strings.Contains(output, "Found 3 services") {
		t.Error("Expected service count in output")
	}
	
	if !strings.Contains(output, "nginx") {
		t.Error("Expected nginx service in output")
	}
	
	if !strings.Contains(output, "Systemd Services") {
		t.Error("Expected systemd section in output")
	}
	
	if !strings.Contains(output, "Docker Services") {
		t.Error("Expected docker section in output")
	}
}

func TestServer_FormatServiceInfo(t *testing.T) {
	logger := logrus.New()
	server := NewServer(logger)
	
	service := types.ServiceInfo{
		Name:        "nginx",
		Type:        types.ServiceTypeSystemd,
		Status:      types.StatusActive,
		Description: "Web server",
		PID:         1234,
	}
	
	output := server.formatServiceInfo(service)
	
	expectedContent := []string{
		"nginx", "systemd", "active", "Web server", "1234",
	}
	
	for _, content := range expectedContent {
		if !strings.Contains(output, content) {
			t.Errorf("Expected '%s' in service info output", content)
		}
	}
}

func TestServer_CreateResponses(t *testing.T) {
	logger := logrus.New()
	server := NewServer(logger)
	
	// Test success response
	successResponse := server.createSuccessResponse(1, "test result")
	if successResponse.ID != 1 {
		t.Error("Expected ID 1 in success response")
	}
	if successResponse.Result != "test result" {
		t.Error("Expected 'test result' in success response")
	}
	if successResponse.Error != nil {
		t.Error("Expected no error in success response")
	}
	
	// Test error response
	errorResponse := server.createErrorResponse(2, types.InvalidParams, "test error", nil)
	if errorResponse.ID != 2 {
		t.Error("Expected ID 2 in error response")
	}
	if errorResponse.Error == nil {
		t.Error("Expected error in error response")
	}
	if errorResponse.Error.Code != types.InvalidParams {
		t.Errorf("Expected error code %d, got %d", types.InvalidParams, errorResponse.Error.Code)
	}
	if errorResponse.Error.Message != "test error" {
		t.Errorf("Expected error message 'test error', got %s", errorResponse.Error.Message)
	}
	
	// Test tool error response
	toolErrorResponse := server.createToolErrorResponse(3, "tool error")
	if toolErrorResponse.ID != 3 {
		t.Error("Expected ID 3 in tool error response")
	}
	result, ok := toolErrorResponse.Result.(types.CallToolResult)
	if !ok {
		t.Error("Expected CallToolResult in tool error response")
	}
	if !result.IsError {
		t.Error("Expected IsError to be true in tool error response")
	}
	if !strings.Contains(result.Content[0].Text, "Error: tool error") {
		t.Error("Expected error text in tool error response")
	}
}

// Benchmark tests
func BenchmarkServer_HandleListTools(b *testing.B) {
	logger := logrus.New()
	server := NewServer(logger)
	
	request := &types.MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		server.handleListTools(request)
	}
}

func BenchmarkServer_HandleCallTool_ListServices(b *testing.B) {
	logger := logrus.New()
	server := NewServer(logger)
	
	request := &types.MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params: types.CallToolParams{
			Name:      "list_services",
			Arguments: map[string]interface{}{},
		},
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		server.handleCallTool(request)
	}
}