package test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMCPStdio(t *testing.T) {
	// 构建服务器二进制文件路径
	serverBinary := filepath.Join("..", "mcp-server")
	
	// 检查二进制文件是否存在
	if _, err := os.Stat(serverBinary); os.IsNotExist(err) {
		t.Fatalf("Server binary not found at %s. Please run 'go build -o mcp-server cmd/server/main.go' first", serverBinary)
	}

	helper := NewTestHelper(serverBinary)
	defer helper.StopAllServers()

	t.Run("StartMCPServer", func(t *testing.T) {
		server, err := helper.StartServer("mcp", 0) // MCP stdio 不需要端口
		if err != nil {
			t.Fatalf("Failed to start MCP server: %v", err)
		}

		if server.mode != "mcp" {
			t.Errorf("Expected mode 'mcp', got '%s'", server.mode)
		}

		// 验证服务器已启动
		if !server.started {
			t.Error("Server should be marked as started")
		}
	})

	t.Run("InitializeMCP", func(t *testing.T) {
		server, _ := helper.StartServer("mcp", 0)

		// 发送初始化请求
		initRequest := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "initialize",
			"params": map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"capabilities":    map[string]interface{}{},
				"clientInfo": map[string]interface{}{
					"name":    "test-client",
					"version": "1.0.0",
				},
			},
		}

		err := server.SendMCPMessage(initRequest)
		if err != nil {
			t.Fatalf("Failed to send initialize message: %v", err)
		}

		// 读取响应
		response, err := server.ReadMCPResponse()
		if err != nil {
			t.Fatalf("Failed to read initialize response: %v", err)
		}

		// 验证响应
		if response["jsonrpc"] != "2.0" {
			t.Errorf("Expected jsonrpc '2.0', got '%v'", response["jsonrpc"])
		}

		if response["id"] != float64(1) { // JSON 数字被解析为 float64
			t.Errorf("Expected id 1, got '%v'", response["id"])
		}

		result, ok := response["result"].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected result to be an object, got %T", response["result"])
		}

		if result["protocolVersion"] != "2024-11-05" {
			t.Errorf("Expected protocolVersion '2024-11-05', got '%v'", result["protocolVersion"])
		}

		capabilities, ok := result["capabilities"].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected capabilities to be an object, got %T", result["capabilities"])
		}

		// 验证工具能力
		tools, ok := capabilities["tools"].(map[string]interface{})
		if !ok {
			t.Errorf("Expected tools capability, got %T", capabilities["tools"])
		} else {
			if listChanged, exists := tools["listChanged"]; exists && listChanged != false {
				t.Errorf("Expected tools.listChanged to be false, got %v", listChanged)
			}
		}

		// 验证提示词能力
		prompts, ok := capabilities["prompts"].(map[string]interface{})
		if !ok {
			t.Errorf("Expected prompts capability, got %T", capabilities["prompts"])
		} else {
			if listChanged, exists := prompts["listChanged"]; exists && listChanged != false {
				t.Errorf("Expected prompts.listChanged to be false, got %v", listChanged)
			}
		}
	})

	t.Run("ListTools", func(t *testing.T) {
		server, _ := helper.StartServer("mcp", 0)

		// 先发送初始化请求
		initRequest := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "initialize",
		}
		server.SendMCPMessage(initRequest)
		server.ReadMCPResponse() // 忽略初始化响应

		// 发送工具列表请求
		listToolsRequest := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      2,
			"method":  "tools/list",
		}

		err := server.SendMCPMessage(listToolsRequest)
		if err != nil {
			t.Fatalf("Failed to send tools/list message: %v", err)
		}

		// 读取响应
		response, err := server.ReadMCPResponse()
		if err != nil {
			t.Fatalf("Failed to read tools/list response: %v", err)
		}

		// 验证响应
		result, ok := response["result"].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected result to be an object, got %T", response["result"])
		}

		tools, ok := result["tools"].([]interface{})
		if !ok {
			t.Fatalf("Expected tools to be an array, got %T", result["tools"])
		}

		// 验证工具数量（应该有8个工具）
		expectedTools := []string{
			"list_services", "get_service_status", "start_service",
			"stop_service", "restart_service", "enable_service",
			"disable_service", "get_docker_logs",
		}

		if len(tools) != len(expectedTools) {
			t.Errorf("Expected %d tools, got %d", len(expectedTools), len(tools))
		}

		// 验证每个工具都存在
		toolNames := make(map[string]bool)
		for _, tool := range tools {
			toolObj, ok := tool.(map[string]interface{})
			if !ok {
				t.Errorf("Expected tool to be an object, got %T", tool)
				continue
			}
			name, ok := toolObj["name"].(string)
			if !ok {
				t.Errorf("Expected tool name to be a string, got %T", toolObj["name"])
				continue
			}
			toolNames[name] = true
		}

		for _, expectedTool := range expectedTools {
			if !toolNames[expectedTool] {
				t.Errorf("Expected tool '%s' not found", expectedTool)
			}
		}
	})

	t.Run("CallListServicesTool", func(t *testing.T) {
		server, _ := helper.StartServer("mcp", 0)

		// 初始化
		initRequest := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "initialize",
		}
		server.SendMCPMessage(initRequest)
		server.ReadMCPResponse()

		// 调用 list_services 工具
		callToolRequest := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      2,
			"method":  "tools/call",
			"params": map[string]interface{}{
				"name": "list_services",
				"arguments": map[string]interface{}{
					// 不指定 service_type，获取所有服务
				},
			},
		}

		err := server.SendMCPMessage(callToolRequest)
		if err != nil {
			t.Fatalf("Failed to send tools/call message: %v", err)
		}

		// 读取响应
		response, err := server.ReadMCPResponse()
		if err != nil {
			t.Fatalf("Failed to read tools/call response: %v", err)
		}

		// 验证响应
		result, ok := response["result"].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected result to be an object, got %T", response["result"])
		}

		content, ok := result["content"].([]interface{})
		if !ok {
			t.Fatalf("Expected content to be an array, got %T", result["content"])
		}

		if len(content) == 0 {
			t.Error("Expected at least one content item")
		} else {
			firstContent, ok := content[0].(map[string]interface{})
			if !ok {
				t.Errorf("Expected content item to be an object, got %T", content[0])
			} else {
				if firstContent["type"] != "text" {
					t.Errorf("Expected content type 'text', got '%v'", firstContent["type"])
				}
				
				text, ok := firstContent["text"].(string)
				if !ok {
					t.Errorf("Expected text to be a string, got %T", firstContent["text"])
				} else if text == "" {
					t.Error("Expected non-empty text content")
				}
			}
		}
	})

	t.Run("ListPrompts", func(t *testing.T) {
		server, _ := helper.StartServer("mcp", 0)

		// 初始化
		initRequest := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "initialize",
		}
		server.SendMCPMessage(initRequest)
		server.ReadMCPResponse()

		// 发送提示词列表请求
		listPromptsRequest := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      2,
			"method":  "prompts/list",
		}

		err := server.SendMCPMessage(listPromptsRequest)
		if err != nil {
			t.Fatalf("Failed to send prompts/list message: %v", err)
		}

		// 读取响应
		response, err := server.ReadMCPResponse()
		if err != nil {
			t.Fatalf("Failed to read prompts/list response: %v", err)
		}

		// 验证响应
		result, ok := response["result"].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected result to be an object, got %T", response["result"])
		}

		prompts, ok := result["prompts"].([]interface{})
		if !ok {
			t.Fatalf("Expected prompts to be an array, got %T", result["prompts"])
		}

		// 验证提示词数量（应该有2个提示词）
		expectedPrompts := []string{
			"service_management_help",
			"service_troubleshooting",
		}

		if len(prompts) != len(expectedPrompts) {
			t.Errorf("Expected %d prompts, got %d", len(expectedPrompts), len(prompts))
		}

		// 验证每个提示词都存在
		promptNames := make(map[string]bool)
		for _, prompt := range prompts {
			promptObj, ok := prompt.(map[string]interface{})
			if !ok {
				t.Errorf("Expected prompt to be an object, got %T", prompt)
				continue
			}
			name, ok := promptObj["name"].(string)
			if !ok {
				t.Errorf("Expected prompt name to be a string, got %T", promptObj["name"])
				continue
			}
			promptNames[name] = true
		}

		for _, expectedPrompt := range expectedPrompts {
			if !promptNames[expectedPrompt] {
				t.Errorf("Expected prompt '%s' not found", expectedPrompt)
			}
		}
	})

	t.Run("InvalidMethod", func(t *testing.T) {
		server, _ := helper.StartServer("mcp", 0)

		// 发送无效方法请求
		invalidRequest := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "invalid/method",
		}

		err := server.SendMCPMessage(invalidRequest)
		if err != nil {
			t.Fatalf("Failed to send invalid method message: %v", err)
		}

		// 读取响应
		response, err := server.ReadMCPResponse()
		if err != nil {
			t.Fatalf("Failed to read invalid method response: %v", err)
		}

		// 验证错误响应
		if response["error"] == nil {
			t.Error("Expected error response for invalid method")
		}

		errorObj, ok := response["error"].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected error to be an object, got %T", response["error"])
		}

		// 验证错误码（-32601 表示方法未找到）
		code, ok := errorObj["code"].(float64)
		if !ok {
			t.Errorf("Expected error code to be a number, got %T", errorObj["code"])
		} else if code != -32601 {
			t.Errorf("Expected error code -32601, got %v", code)
		}
	})
}

func TestMCPStdioWithTimeout(t *testing.T) {
	// 使用更长的超时时间用于集成测试
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	serverBinary := filepath.Join("..", "mcp-server")
	helper := NewTestHelper(serverBinary)
	defer helper.StopAllServers()

	server, err := helper.StartServer("mcp", 0)
	if err != nil {
		t.Fatalf("Failed to start MCP server: %v", err)
	}

	// 在后台读取日志
	go func() {
		LogOutput("MCP-STDERR", server.GetStderr())
	}()

	// 执行一系列操作来测试服务器稳定性
	for i := 0; i < 5; i++ {
		t.Logf("Iteration %d", i+1)

		// 初始化
		initRequest := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      i*10 + 1,
			"method":  "initialize",
		}

		err := server.SendMCPMessage(initRequest)
		if err != nil {
			t.Fatalf("Failed to send initialize message (iteration %d): %v", i+1, err)
		}

		response, err := server.ReadMCPResponse()
		if err != nil {
			t.Fatalf("Failed to read initialize response (iteration %d): %v", i+1, err)
		}

		if response["error"] != nil {
			errorBytes, _ := json.Marshal(response["error"])
			t.Fatalf("Initialize failed (iteration %d): %s", i+1, string(errorBytes))
		}

		// 列出工具
		listToolsRequest := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      i*10 + 2,
			"method":  "tools/list",
		}

		err = server.SendMCPMessage(listToolsRequest)
		if err != nil {
			t.Fatalf("Failed to send tools/list message (iteration %d): %v", i+1, err)
		}

		response, err = server.ReadMCPResponse()
		if err != nil {
			t.Fatalf("Failed to read tools/list response (iteration %d): %v", i+1, err)
		}

		if response["error"] != nil {
			errorBytes, _ := json.Marshal(response["error"])
			t.Fatalf("Tools/list failed (iteration %d): %s", i+1, string(errorBytes))
		}

		// 短暂等待
		time.Sleep(100 * time.Millisecond)
	}

	t.Log("Multiple iterations completed successfully")
}