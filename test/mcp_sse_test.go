package test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// SSEClient SSE客户端
type SSEClient struct {
	client    *http.Client
	baseURL   string
	sessionID string
}

// NewSSEClient 创建SSE客户端
func NewSSEClient(baseURL string) *SSEClient {
	return &SSEClient{
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: baseURL,
	}
}

// Connect 连接SSE端点
func (c *SSEClient) Connect(ctx context.Context) (<-chan SSEEvent, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/sse", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// 获取会话ID
	c.sessionID = resp.Header.Get("X-MCP-Client-ID")
	if c.sessionID == "" {
		resp.Body.Close()
		return nil, fmt.Errorf("no session ID received")
	}

	eventChan := make(chan SSEEvent, 10)

	go func() {
		defer resp.Body.Close()
		defer close(eventChan)

		scanner := bufio.NewScanner(resp.Body)
		var eventType, data string

		for scanner.Scan() {
			line := scanner.Text()
			
			if line == "" {
				// 空行表示事件结束
				if eventType != "" && data != "" {
					eventChan <- SSEEvent{Type: eventType, Data: data}
					eventType, data = "", ""
				}
				continue
			}

			if strings.HasPrefix(line, "event: ") {
				eventType = strings.TrimPrefix(line, "event: ")
			} else if strings.HasPrefix(line, "data: ") {
				data = strings.TrimPrefix(line, "data: ")
			}
		}
	}()

	return eventChan, nil
}

// SendMessage 发送消息
func (c *SSEClient) SendMessage(message map[string]interface{}) error {
	if c.sessionID == "" {
		return fmt.Errorf("not connected")
	}

	jsonData, err := json.Marshal(message)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/message?session=%s", c.baseURL, c.sessionID)
	resp, err := c.client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// SSEEvent SSE事件
type SSEEvent struct {
	Type string
	Data string
}

func TestMCPSSE(t *testing.T) {
	serverBinary := filepath.Join("..", "mcp-server")
	
	if _, err := os.Stat(serverBinary); os.IsNotExist(err) {
		t.Fatalf("Server binary not found at %s. Please run 'go build -o mcp-server cmd/server/main.go' first", serverBinary)
	}

	helper := NewTestHelper(serverBinary)
	defer helper.StopAllServers()

	const ssePort = 8083

	t.Run("StartSSEServer", func(t *testing.T) {
		server, err := helper.StartServer("mcp-http", ssePort)
		if err != nil {
			t.Fatalf("Failed to start MCP HTTP server: %v", err)
		}

		if server.mode != "mcp-http" {
			t.Errorf("Expected mode 'mcp-http', got '%s'", server.mode)
		}

		if server.port != ssePort {
			t.Errorf("Expected port %d, got %d", ssePort, server.port)
		}
	})

	t.Run("SSEConnection", func(t *testing.T) {
		_, err := helper.StartServer("mcp-http", ssePort)
		if err != nil {
			t.Fatalf("Failed to start MCP HTTP server: %v", err)
		}

		client := NewSSEClient(fmt.Sprintf("http://127.0.0.1:%d", ssePort))
		
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		eventChan, err := client.Connect(ctx)
		if err != nil {
			t.Fatalf("Failed to connect to SSE: %v", err)
		}

		// 应该接收到初始连接事件
		select {
		case event := <-eventChan:
			if event.Type != "connected" {
				t.Errorf("Expected 'connected' event, got '%s'", event.Type)
			}

			var eventData map[string]interface{}
			err := json.Unmarshal([]byte(event.Data), &eventData)
			if err != nil {
				t.Fatalf("Failed to unmarshal event data: %v", err)
			}

			if eventData["clientId"] == nil {
				t.Error("Expected clientId in connected event")
			}

			t.Logf("Connected with client ID: %v", eventData["clientId"])

		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for connected event")
		}

		// 检查客户端会话ID是否设置
		if client.sessionID == "" {
			t.Error("Expected session ID to be set")
		}

		t.Logf("Session ID: %s", client.sessionID)
	})

	t.Run("MCPInitialize", func(t *testing.T) {
		_, err := helper.StartServer("mcp-http", ssePort)
		if err != nil {
			t.Fatalf("Failed to start MCP HTTP server: %v", err)
		}

		client := NewSSEClient(fmt.Sprintf("http://127.0.0.1:%d", ssePort))
		
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		eventChan, err := client.Connect(ctx)
		if err != nil {
			t.Fatalf("Failed to connect to SSE: %v", err)
		}

		// 等待连接事件
		<-eventChan

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

		err = client.SendMessage(initRequest)
		if err != nil {
			t.Fatalf("Failed to send initialize message: %v", err)
		}

		// 等待响应，跳过非消息事件
		var response MCPResponse
		for {
			select {
			case event := <-eventChan:
				if event.Type == "message" {
					err := json.Unmarshal([]byte(event.Data), &response)
					if err != nil {
						t.Fatalf("Failed to unmarshal response: %v", err)
					}
					goto initializeResponseReceived
				}
				// 跳过其他事件类型（如endpoint事件）
				t.Logf("Skipping event type: %s", event.Type)
			case <-time.After(10 * time.Second):
				t.Fatal("Timeout waiting for initialize message event")
			}
		}
		initializeResponseReceived:

		if response.JSONRPC != "2.0" {
			t.Errorf("Expected jsonrpc '2.0', got '%s'", response.JSONRPC)
		}

		if response.Error != nil {
			t.Fatalf("Initialize failed with error: %+v", response.Error)
		}

		result, ok := response.Result.(map[string]interface{})
		if !ok {
			t.Fatalf("Expected result to be an object, got %T", response.Result)
		}

		if result["protocolVersion"] != "2024-11-05" {
			t.Errorf("Expected protocolVersion '2024-11-05', got '%v'", result["protocolVersion"])
		}

		t.Log("Initialize successful")
	})

	t.Run("MCPListTools", func(t *testing.T) {
		_, err := helper.StartServer("mcp-http", ssePort)
		if err != nil {
			t.Fatalf("Failed to start MCP HTTP server: %v", err)
		}

		client := NewSSEClient(fmt.Sprintf("http://127.0.0.1:%d", ssePort))
		
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		eventChan, err := client.Connect(ctx)
		if err != nil {
			t.Fatalf("Failed to connect to SSE: %v", err)
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
			t.Fatalf("Failed to send tools/list message: %v", err)
		}

		// 等待响应，跳过非消息事件
		var response MCPResponse
		for {
			select {
			case event := <-eventChan:
				if event.Type == "message" {
					err := json.Unmarshal([]byte(event.Data), &response)
					if err != nil {
						t.Fatalf("Failed to unmarshal response: %v", err)
					}
					goto toolsListResponseReceived
				}
				// 跳过其他事件类型（如endpoint事件）
				t.Logf("Skipping event type: %s", event.Type)
			case <-time.After(10 * time.Second):
				t.Fatal("Timeout waiting for tools/list message event")
			}
		}
		toolsListResponseReceived:

		if response.Error != nil {
			t.Fatalf("Tools/list failed with error: %+v", response.Error)
		}

			result, ok := response.Result.(map[string]interface{})
			if !ok {
				t.Fatalf("Expected result to be an object, got %T", response.Result)
			}

			tools, ok := result["tools"].([]interface{})
			if !ok {
				t.Fatalf("Expected tools to be an array, got %T", result["tools"])
			}

			if len(tools) == 0 {
				t.Error("Expected at least one tool")
			}

			t.Logf("Found %d tools", len(tools))

			// 验证第一个工具的结构
			if len(tools) > 0 {
				firstTool, ok := tools[0].(map[string]interface{})
				if !ok {
					t.Errorf("Expected tool to be an object, got %T", tools[0])
				} else {
					expectedFields := []string{"name", "description", "inputSchema"}
					for _, field := range expectedFields {
						if _, exists := firstTool[field]; !exists {
							t.Errorf("Expected field '%s' not found in tool", field)
						}
					}
				}
		}
	})

	t.Run("MCPCallTool", func(t *testing.T) {
		_, err := helper.StartServer("mcp-http", ssePort)
		if err != nil {
			t.Fatalf("Failed to start MCP HTTP server: %v", err)
		}

		client := NewSSEClient(fmt.Sprintf("http://127.0.0.1:%d", ssePort))
		
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		eventChan, err := client.Connect(ctx)
		if err != nil {
			t.Fatalf("Failed to connect to SSE: %v", err)
		}

		// 等待连接事件
		<-eventChan

		// 调用 list_services 工具
		callToolRequest := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "tools/call",
			"params": map[string]interface{}{
				"name": "list_services",
				"arguments": map[string]interface{}{
					// 不指定 service_type，获取所有服务
				},
			},
		}

		err = client.SendMessage(callToolRequest)
		if err != nil {
			t.Fatalf("Failed to send tools/call message: %v", err)
		}

		// 等待响应，跳过非消息事件
		var response MCPResponse
		for {
			select {
			case event := <-eventChan:
				if event.Type == "message" {
					err := json.Unmarshal([]byte(event.Data), &response)
					if err != nil {
						t.Fatalf("Failed to unmarshal response: %v", err)
					}
					goto toolsCallResponseReceived
				}
				// 跳过其他事件类型（如endpoint事件）
				t.Logf("Skipping event type: %s", event.Type)
			case <-time.After(10 * time.Second):
				t.Fatal("Timeout waiting for tools/call message event")
			}
		}
		toolsCallResponseReceived:

		if response.Error != nil {
			t.Fatalf("Tools/call failed with error: %+v", response.Error)
		}

			result, ok := response.Result.(map[string]interface{})
			if !ok {
				t.Fatalf("Expected result to be an object, got %T", response.Result)
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
					} else {
						t.Logf("Tool response: %s", text[:min(100, len(text))])
					}
				}
		}
	})

	t.Run("HeartbeatEvents", func(t *testing.T) {
		_, err := helper.StartServer("mcp-http", ssePort)
		if err != nil {
			t.Fatalf("Failed to start MCP HTTP server: %v", err)
		}

		client := NewSSEClient(fmt.Sprintf("http://127.0.0.1:%d", ssePort))
		
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()

		eventChan, err := client.Connect(ctx)
		if err != nil {
			t.Fatalf("Failed to connect to SSE: %v", err)
		}

		// 等待连接事件
		<-eventChan

		// 等待心跳事件（应该在30秒后出现）
		heartbeatReceived := false
		startTime := time.Now()

		for {
			select {
			case event := <-eventChan:
				if event.Type == "heartbeat" {
					heartbeatReceived = true
					t.Log("Heartbeat event received")
					
					var eventData map[string]interface{}
					err := json.Unmarshal([]byte(event.Data), &eventData)
					if err != nil {
						t.Errorf("Failed to unmarshal heartbeat data: %v", err)
					} else if eventData["timestamp"] == nil {
						t.Error("Expected timestamp in heartbeat event")
					}

					// 收到心跳后就可以返回了
					return
				}

			case <-time.After(35 * time.Second):
				if !heartbeatReceived {
					t.Fatal("No heartbeat received within timeout")
				}
				return

			case <-ctx.Done():
				if !heartbeatReceived {
					elapsed := time.Since(startTime)
					t.Fatalf("Context cancelled, no heartbeat received after %v", elapsed)
				}
				return
			}
		}
	})
}

func TestMCPSSEMultipleClients(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping multiple clients test in short mode")
	}

	serverBinary := filepath.Join("..", "mcp-server")
	helper := NewTestHelper(serverBinary)
	defer helper.StopAllServers()

	const ssePort = 8084

	server, err := helper.StartServer("mcp-http", ssePort)
	if err != nil {
		t.Fatalf("Failed to start MCP HTTP server: %v", err)
	}

	// 在后台读取日志
	go LogOutput("SSE-STDERR", server.GetStderr())

	const numClients = 3
	clientChans := make([]<-chan SSEEvent, numClients)
	clients := make([]*SSEClient, numClients)

	// 连接多个客户端
	for i := 0; i < numClients; i++ {
		client := NewSSEClient(fmt.Sprintf("http://127.0.0.1:%d", ssePort))
		clients[i] = client
		
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		eventChan, err := client.Connect(ctx)
		if err != nil {
			t.Fatalf("Failed to connect client %d: %v", i, err)
		}

		clientChans[i] = eventChan

		// 等待连接事件
		select {
		case <-eventChan:
			t.Logf("Client %d connected with session ID: %s", i, client.sessionID)
		case <-time.After(5 * time.Second):
			t.Fatalf("Client %d connection timeout", i)
		}
	}

	// 让每个客户端发送请求
	for i, client := range clients {
		t.Logf("Sending request from client %d", i)

		listToolsRequest := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      i + 1,
			"method":  "tools/list",
		}

		err := client.SendMessage(listToolsRequest)
		if err != nil {
			t.Errorf("Client %d failed to send message: %v", i, err)
			continue
		}

		// 等待响应，跳过非消息事件
		var response MCPResponse
		responseReceived := false
		hasError := false
		for !responseReceived && !hasError {
			select {
			case event := <-clientChans[i]:
				if event.Type == "message" {
					err := json.Unmarshal([]byte(event.Data), &response)
					if err != nil {
						t.Errorf("Client %d: Failed to unmarshal response: %v", i, err)
						hasError = true
					} else {
						responseReceived = true
					}
				} else {
					// 跳过其他事件类型（如endpoint事件）
					t.Logf("Client %d: Skipping event type: %s", i, event.Type)
				}
			case <-time.After(10 * time.Second):
				t.Errorf("Client %d: Timeout waiting for message event", i)
				hasError = true
			}
		}

		if hasError || !responseReceived {
			continue
		}

		if response.Error != nil {
			t.Errorf("Client %d: Request failed with error: %+v", i, response.Error)
			continue
		}

		t.Logf("Client %d received successful response", i)
	}

	t.Log("Multiple clients test completed successfully")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}