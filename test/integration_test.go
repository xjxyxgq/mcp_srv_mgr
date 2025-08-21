package test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"
)

// TestAllProtocolsIntegration 集成测试所有四种协议
func TestAllProtocolsIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	serverBinary := filepath.Join("..", "mcp-server")
	
	if _, err := os.Stat(serverBinary); os.IsNotExist(err) {
		t.Fatalf("Server binary not found at %s. Please run 'go build -o mcp-server cmd/server/main.go' first", serverBinary)
	}

	helper := NewTestHelper(serverBinary)
	defer helper.StopAllServers()

	// 定义所有协议及其端口
	protocols := map[string]int{
		"mcp":            0,    // stdio 不需要端口
		"http":           9001,
		"mcp-http":       9002,
		"mcp-streamable": 9003,
	}

	t.Run("StartAllServers", func(t *testing.T) {
		// 启动所有协议的服务器
		for protocol, port := range protocols {
			t.Run(fmt.Sprintf("Start_%s", protocol), func(t *testing.T) {
				server, err := helper.StartServer(protocol, port)
				if err != nil {
					t.Fatalf("Failed to start %s server: %v", protocol, err)
				}

				if server.mode != protocol {
					t.Errorf("Expected mode '%s', got '%s'", protocol, server.mode)
				}

				t.Logf("Successfully started %s server", protocol)
			})
		}
	})

	t.Run("ConcurrentProtocolTest", func(t *testing.T) {
		var wg sync.WaitGroup
		results := make(chan TestResult, len(protocols))

		// 并发测试所有协议
		for protocol, port := range protocols {
			wg.Add(1)
			go func(p string, port int) {
				defer wg.Done()
				
				var err error
				switch p {
				case "mcp":
					err = testMCPStdioProtocol(t, helper)
				case "http":
					err = testHTTPProtocol(t, port)
				case "mcp-http":
					err = testMCPSSEProtocol(t, port)
				case "mcp-streamable":
					err = testMCPStreamableProtocol(t, port)
				}

				results <- TestResult{Protocol: p, Error: err}
			}(protocol, port)
		}

		// 等待所有测试完成
		go func() {
			wg.Wait()
			close(results)
		}()

		// 收集结果
		var failures []TestResult
		successCount := 0

		for result := range results {
			if result.Error != nil {
				failures = append(failures, result)
				t.Errorf("Protocol %s failed: %v", result.Protocol, result.Error)
			} else {
				successCount++
				t.Logf("Protocol %s passed", result.Protocol)
			}
		}

		// 报告结果
		t.Logf("Integration test results: %d/%d protocols passed", successCount, len(protocols))

		if len(failures) > 0 {
			t.Errorf("Failed protocols:")
			for _, failure := range failures {
				t.Errorf("  - %s: %v", failure.Protocol, failure.Error)
			}
		} else {
			t.Log("All protocols passed integration test!")
		}
	})

	t.Run("ResourceUsageTest", func(t *testing.T) {
		// 检查服务器资源使用情况
		for protocol := range protocols {
			server, exists := helper.servers[protocol]
			if !exists {
				continue
			}

			if server.process == nil {
				continue
			}

			// 简单检查进程是否仍在运行
			// 使用信号0来测试进程是否存在，不会实际发送信号
			if err := server.process.Signal(syscall.Signal(0)); err != nil {
				t.Errorf("Server %s process appears to be dead: %v", protocol, err)
			} else {
				t.Logf("Server %s is running normally", protocol)
			}
		}
	})

	t.Run("GracefulShutdown", func(t *testing.T) {
		// 测试优雅关闭
		shutdownResults := make(map[string]error)

		for protocol := range protocols {
			err := helper.StopServer(protocol)
			shutdownResults[protocol] = err
			
			if err != nil {
				t.Errorf("Failed to stop %s server gracefully: %v", protocol, err)
			} else {
				t.Logf("Successfully stopped %s server", protocol)
			}
		}

		// 确保所有服务器都已停止
		time.Sleep(2 * time.Second)

		for protocol, err := range shutdownResults {
			if err == nil {
				t.Logf("Protocol %s shutdown: OK", protocol)
			} else {
				t.Errorf("Protocol %s shutdown: FAILED - %v", protocol, err)
			}
		}
	})
}

// TestResult 测试结果
type TestResult struct {
	Protocol string
	Error    error
}

// testMCPStdioProtocol 测试MCP stdio协议
func testMCPStdioProtocol(t *testing.T, helper *TestHelper) error {
	server, exists := helper.servers["mcp"]
	if !exists {
		return fmt.Errorf("MCP server not found")
	}

	// 发送简单的初始化请求
	initRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
	}

	err := server.SendMCPMessage(initRequest)
	if err != nil {
		return fmt.Errorf("failed to send message: %v", err)
	}

	response, err := server.ReadMCPResponse()
	if err != nil {
		return fmt.Errorf("failed to read response: %v", err)
	}

	if response["error"] != nil {
		return fmt.Errorf("initialize failed: %v", response["error"])
	}

	return nil
}

// testHTTPProtocol 测试HTTP协议
func testHTTPProtocol(t *testing.T, port int) error {
	client := NewHTTPClient(fmt.Sprintf("http://127.0.0.1:%d", port))
	
	resp, err := client.Get("/health")
	if err != nil {
		return fmt.Errorf("health check failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

// testMCPSSEProtocol 测试MCP SSE协议
func testMCPSSEProtocol(t *testing.T, port int) error {
	client := NewSSEClient(fmt.Sprintf("http://127.0.0.1:%d", port))
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	eventChan, err := client.Connect(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect: %v", err)
	}

	// 等待连接事件
	select {
	case event := <-eventChan:
		if event.Type != "connected" {
			return fmt.Errorf("expected connected event, got: %s", event.Type)
		}
	case <-time.After(5 * time.Second):
		return fmt.Errorf("timeout waiting for connected event")
	}

	return nil
}

// testMCPStreamableProtocol 测试MCP Streamable协议
func testMCPStreamableProtocol(t *testing.T, port int) error {
	client := NewStreamableClient(fmt.Sprintf("http://127.0.0.1:%d", port))

	// 测试单个请求
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
	}

	response, err := client.SendSingleRequest(request)
	if err != nil {
		return fmt.Errorf("single request failed: %v", err)
	}

	if response.Error != nil {
		return fmt.Errorf("initialize failed: %+v", response.Error)
	}

	return nil
}

// TestProtocolCompatibility 测试协议兼容性
func TestProtocolCompatibility(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping compatibility test in short mode")
	}

	serverBinary := filepath.Join("..", "mcp-server")
	helper := NewTestHelper(serverBinary)
	defer helper.StopAllServers()

	// 测试所有MCP协议是否返回相同的工具列表
	mcpProtocols := map[string]int{
		"mcp":            0,
		"mcp-http":       9004,
		"mcp-streamable": 9005,
	}

	toolLists := make(map[string][]string)

	for protocol, port := range mcpProtocols {
		t.Run(fmt.Sprintf("GetTools_%s", protocol), func(t *testing.T) {
			server, err := helper.StartServer(protocol, port)
			if err != nil {
				t.Fatalf("Failed to start %s server: %v", protocol, err)
			}

			var tools []string
			switch protocol {
			case "mcp":
				tools, err = getMCPStdioTools(server)
			case "mcp-http":
				tools, err = getMCPSSETools(port)
			case "mcp-streamable":
				tools, err = getMCPStreamableTools(port)
			}

			if err != nil {
				t.Fatalf("Failed to get tools from %s: %v", protocol, err)
			}

			toolLists[protocol] = tools
			t.Logf("Protocol %s returned %d tools", protocol, len(tools))
		})
	}

	// 比较工具列表
	t.Run("CompareToolLists", func(t *testing.T) {
		var referenceTool []string
		var referenceProtocol string

		// 获取参考工具列表
		for protocol, tools := range toolLists {
			if len(tools) > 0 {
				referenceTool = tools
				referenceProtocol = protocol
				break
			}
		}

		if len(referenceTool) == 0 {
			t.Fatal("No protocol returned any tools")
		}

		t.Logf("Using %s as reference with %d tools", referenceProtocol, len(referenceTool))

		// 比较其他协议的工具列表
		for protocol, tools := range toolLists {
			if protocol == referenceProtocol {
				continue
			}

			if len(tools) != len(referenceTool) {
				t.Errorf("Protocol %s returned %d tools, expected %d", protocol, len(tools), len(referenceTool))
			}

			// 检查工具名称是否一致
			toolSet := make(map[string]bool)
			for _, tool := range tools {
				toolSet[tool] = true
			}

			for _, expectedTool := range referenceTool {
				if !toolSet[expectedTool] {
					t.Errorf("Protocol %s missing tool: %s", protocol, expectedTool)
				}
			}
		}

		t.Log("Tool compatibility check completed")
	})
}

// 辅助函数获取不同协议的工具列表
func getMCPStdioTools(server *ServerInstance) ([]string, error) {
	// 初始化
	initRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
	}
	server.SendMCPMessage(initRequest)
	server.ReadMCPResponse()

	// 获取工具列表
	listToolsRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
	}
	
	server.SendMCPMessage(listToolsRequest)
	response, err := server.ReadMCPResponse()
	if err != nil {
		return nil, err
	}

	result, ok := response["result"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid result format")
	}

	tools, ok := result["tools"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid tools format")
	}

	var toolNames []string
	for _, tool := range tools {
		toolObj, ok := tool.(map[string]interface{})
		if !ok {
			continue
		}
		name, ok := toolObj["name"].(string)
		if !ok {
			continue
		}
		toolNames = append(toolNames, name)
	}

	return toolNames, nil
}

func getMCPSSETools(port int) ([]string, error) {
	client := NewSSEClient(fmt.Sprintf("http://127.0.0.1:%d", port))
	
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	eventChan, err := client.Connect(ctx)
	if err != nil {
		return nil, err
	}

	// 等待连接事件
	<-eventChan

	// 发送工具列表请求
	listToolsRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
	}

	err = client.SendMessage(listToolsRequest)
	if err != nil {
		return nil, err
	}

	// 等待响应，跳过非消息事件
	var response MCPResponse
	for {
		select {
		case event := <-eventChan:
			if event.Type == "message" {
				err := json.Unmarshal([]byte(event.Data), &response)
				if err != nil {
					return nil, err
				}
				goto toolsListResponseReceived
			}
			// 跳过其他事件类型（如endpoint事件）
		case <-time.After(10 * time.Second):
			return nil, fmt.Errorf("timeout waiting for tools/list message event")
		}
	}
	toolsListResponseReceived:

	result, ok := response.Result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid result format")
	}

	tools, ok := result["tools"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid tools format")
	}

	var toolNames []string
	for _, tool := range tools {
		toolObj, ok := tool.(map[string]interface{})
		if !ok {
			continue
		}
		name, ok := toolObj["name"].(string)
		if !ok {
			continue
		}
		toolNames = append(toolNames, name)
	}

	return toolNames, nil
}

func getMCPStreamableTools(port int) ([]string, error) {
	client := NewStreamableClient(fmt.Sprintf("http://127.0.0.1:%d", port))

	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
	}

	response, err := client.SendSingleRequest(request)
	if err != nil {
		return nil, err
	}

	if response.Error != nil {
		return nil, fmt.Errorf("request failed: %+v", response.Error)
	}

	result, ok := response.Result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid result format")
	}

	tools, ok := result["tools"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid tools format")
	}

	var toolNames []string
	for _, tool := range tools {
		toolObj, ok := tool.(map[string]interface{})
		if !ok {
			continue
		}
		name, ok := toolObj["name"].(string)
		if !ok {
			continue
		}
		toolNames = append(toolNames, name)
	}

	return toolNames, nil
}