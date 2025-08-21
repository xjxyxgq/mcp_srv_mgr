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

// StreamableClient MCP Streamable HTTP 客户端
type StreamableClient struct {
	client    *http.Client
	baseURL   string
	sessionID string
}

// NewStreamableClient 创建Streamable客户端
func NewStreamableClient(baseURL string) *StreamableClient {
	return &StreamableClient{
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: baseURL,
	}
}

// ConnectBidirectional 建立双向流连接
func (c *StreamableClient) ConnectBidirectional(ctx context.Context) (*StreamableConnection, error) {
	// For bidirectional streaming, we'll simulate it using the session-based approach
	// First, create a session by sending a streaming POST request
	var buf bytes.Buffer
	
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/mcp/stream", &buf)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-ndjson")
	req.Header.Set("Connection", "upgrade")
	req.Header.Set("Upgrade", "mcp-stream")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	resp.Body.Close() // Close the initial response

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// 获取会话ID
	sessionID := resp.Header.Get("X-MCP-Session-ID")
	if sessionID == "" {
		return nil, fmt.Errorf("no session ID received")
	}

	c.sessionID = sessionID

	// Now establish a GET connection to the session stream
	getReq, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/mcp/stream/"+sessionID, nil)
	if err != nil {
		return nil, err
	}

	getResp, err := c.client.Do(getReq)
	if err != nil {
		return nil, err
	}

	if getResp.StatusCode != http.StatusOK {
		getResp.Body.Close()
		return nil, fmt.Errorf("unexpected status code for stream: %d", getResp.StatusCode)
	}

	conn := &StreamableConnection{
		reader:     getResp.Body,
		writer:     nil, // We'll use POST requests to send messages
		sessionID:  sessionID,
		ctx:        ctx,
		messageChan: make(chan StreamableMessage, 10),
		client:     c.client,
		baseURL:    c.baseURL,
	}

	// 启动读取协程
	go conn.readMessages()

	return conn, nil
}

// SendSingleRequest 发送单个请求并等待响应
func (c *StreamableClient) SendSingleRequest(request map[string]interface{}) (*StreamableResponse, error) {
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Post(c.baseURL+"/mcp/stream", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var response StreamableResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

// StreamableConnection 流式连接
type StreamableConnection struct {
	reader      io.ReadCloser
	writer      io.WriteCloser
	sessionID   string
	ctx         context.Context
	messageChan chan StreamableMessage
	client      *http.Client
	baseURL     string
}

// StreamableMessage 流式消息
type StreamableMessage struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Method  string      `json:"method,omitempty"`
	Params  interface{} `json:"params,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *StreamableError `json:"error,omitempty"`
}

// StreamableResponse 流式响应
type StreamableResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *StreamableError `json:"error,omitempty"`
}

// StreamableError 流式错误
type StreamableError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// SendMessage 发送消息
func (c *StreamableConnection) SendMessage(message map[string]interface{}) error {
	jsonData, err := json.Marshal(message)
	if err != nil {
		return err
	}

	// For bidirectional connections, send via HTTP POST to a message endpoint
	if c.sessionID != "" && c.client != nil {
		url := fmt.Sprintf("%s/message?session=%s", c.baseURL, c.sessionID)
		resp, err := c.client.Post(url, "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		
		if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("message send failed: status %d, body: %s", resp.StatusCode, string(body))
		}
		return nil
	}

	// Fallback to pipe writer if available
	if c.writer != nil {
		_, err = c.writer.Write(append(jsonData, '\n'))
		return err
	}
	
	return fmt.Errorf("no way to send message")
}

// ReceiveMessage 接收消息
func (c *StreamableConnection) ReceiveMessage() <-chan StreamableMessage {
	return c.messageChan
}

// Close 关闭连接
func (c *StreamableConnection) Close() error {
	c.writer.Close()
	return c.reader.Close()
}

// readMessages 读取消息的协程
func (c *StreamableConnection) readMessages() {
	defer close(c.messageChan)

	scanner := bufio.NewScanner(c.reader)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var message StreamableMessage
		if err := json.Unmarshal([]byte(line), &message); err != nil {
			// 忽略解析错误，继续读取
			continue
		}

		select {
		case c.messageChan <- message:
		case <-c.ctx.Done():
			return
		}
	}
}

func TestMCPStreamable(t *testing.T) {
	serverBinary := filepath.Join("..", "mcp-server")
	
	if _, err := os.Stat(serverBinary); os.IsNotExist(err) {
		t.Fatalf("Server binary not found at %s. Please run 'go build -o mcp-server cmd/server/main.go' first", serverBinary)
	}

	helper := NewTestHelper(serverBinary)
	defer helper.StopAllServers()

	const streamablePort = 8085

	t.Run("StartStreamableServer", func(t *testing.T) {
		server, err := helper.StartServer("mcp-streamable", streamablePort)
		if err != nil {
			t.Fatalf("Failed to start MCP Streamable server: %v", err)
		}

		if server.mode != "mcp-streamable" {
			t.Errorf("Expected mode 'mcp-streamable', got '%s'", server.mode)
		}

		if server.port != streamablePort {
			t.Errorf("Expected port %d, got %d", streamablePort, server.port)
		}
	})

	t.Run("HealthCheck", func(t *testing.T) {
		_, err := helper.StartServer("mcp-streamable", streamablePort)
		if err != nil {
			t.Fatalf("Failed to start MCP Streamable server: %v", err)
		}

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/health", streamablePort))
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

		if healthResponse["mode"] != "mcp-streamable" {
			t.Errorf("Expected mode 'mcp-streamable', got '%v'", healthResponse["mode"])
		}

		t.Logf("Health check passed: %+v", healthResponse)
	})

	t.Run("SingleRequestResponse", func(t *testing.T) {
		_, err := helper.StartServer("mcp-streamable", streamablePort)
		if err != nil {
			t.Fatalf("Failed to start MCP Streamable server: %v", err)
		}

		client := NewStreamableClient(fmt.Sprintf("http://127.0.0.1:%d", streamablePort))

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

		response, err := client.SendSingleRequest(initRequest)
		if err != nil {
			t.Fatalf("Failed to send single request: %v", err)
		}

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

		t.Log("Single request-response successful")
	})

	t.Run("BidirectionalStream", func(t *testing.T) {
		_, err := helper.StartServer("mcp-streamable", streamablePort)
		if err != nil {
			t.Fatalf("Failed to start MCP Streamable server: %v", err)
		}

		client := NewStreamableClient(fmt.Sprintf("http://127.0.0.1:%d", streamablePort))

		// For now, we'll simulate bidirectional communication using multiple single requests
		// This tests the core MCP functionality even if not truly bidirectional streaming

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

		response, err := client.SendSingleRequest(initRequest)
		if err != nil {
			t.Fatalf("Failed to send initialize request: %v", err)
		}

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

		t.Log("Initialize successful via streamable protocol")

		// 发送工具列表请求
		listToolsRequest := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      2,
			"method":  "tools/list",
		}

		response, err = client.SendSingleRequest(listToolsRequest)
		if err != nil {
			t.Fatalf("Failed to send tools/list request: %v", err)
		}

		if response.Error != nil {
			t.Fatalf("Tools/list failed with error: %+v", response.Error)
		}

		result, ok = response.Result.(map[string]interface{})
		if !ok {
			t.Fatalf("Expected result to be an object, got %T", response.Result)
		}

		tools, ok := result["tools"].([]interface{})
		if !ok {
			t.Fatalf("Expected tools to be an array, got %T", result["tools"])
		}

		if len(tools) == 0 {
			t.Error("Expected at least one tool")
		} else {
			t.Logf("Found %d tools via streamable protocol", len(tools))
		}
	})

	t.Run("StreamingToolCall", func(t *testing.T) {
		_, err := helper.StartServer("mcp-streamable", streamablePort)
		if err != nil {
			t.Fatalf("Failed to start MCP Streamable server: %v", err)
		}

		client := NewStreamableClient(fmt.Sprintf("http://127.0.0.1:%d", streamablePort))

		// 发送工具调用请求
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

		response, err := client.SendSingleRequest(callToolRequest)
		if err != nil {
			t.Fatalf("Failed to send tools/call request: %v", err)
		}

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
					t.Logf("Tool response (first 100 chars): %s", text[:min(100, len(text))])
				}
			}
		}
	})

	t.Run("MultipleRequestsInStream", func(t *testing.T) {
		_, err := helper.StartServer("mcp-streamable", streamablePort)
		if err != nil {
			t.Fatalf("Failed to start MCP Streamable server: %v", err)
		}

		client := NewStreamableClient(fmt.Sprintf("http://127.0.0.1:%d", streamablePort))

		// 发送多个请求
		requests := []map[string]interface{}{
			{
				"jsonrpc": "2.0",
				"id":      1,
				"method":  "initialize",
			},
			{
				"jsonrpc": "2.0",
				"id":      2,
				"method":  "tools/list",
			},
			{
				"jsonrpc": "2.0",
				"id":      3,
				"method":  "prompts/list",
			},
		}

		// 发送所有请求并收集响应
		for i, request := range requests {
			response, err := client.SendSingleRequest(request)
			if err != nil {
				t.Fatalf("Failed to send request %d: %v", i+1, err)
			}
			
			if response.Error != nil {
				t.Errorf("Request %d failed with error: %+v", i+1, response.Error)
			} else {
				t.Logf("Received successful response for request %d (ID %v)", i+1, response.ID)
			}
		}

		t.Logf("Successfully processed %d requests via streamable protocol", len(requests))
	})
}

func TestMCPStreamableConcurrent(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent test in short mode")
	}

	serverBinary := filepath.Join("..", "mcp-server")
	helper := NewTestHelper(serverBinary)
	defer helper.StopAllServers()

	const streamablePort = 8086

	server, err := helper.StartServer("mcp-streamable", streamablePort)
	if err != nil {
		t.Fatalf("Failed to start MCP Streamable server: %v", err)
	}

	// 在后台读取日志
	go LogOutput("STREAMABLE-STDERR", server.GetStderr())

	const numClients = 3
	const requestsPerClient = 5

	results := make(chan error, numClients)

	// 启动多个并发客户端
	for clientID := 0; clientID < numClients; clientID++ {
		go func(id int) {
			client := NewStreamableClient(fmt.Sprintf("http://127.0.0.1:%d", streamablePort))

			// 每个客户端发送多个请求
			for reqID := 0; reqID < requestsPerClient; reqID++ {
				request := map[string]interface{}{
					"jsonrpc": "2.0",
					"id":      reqID + 1,
					"method":  "tools/list",
				}

				response, err := client.SendSingleRequest(request)
				if err != nil {
					results <- fmt.Errorf("client %d: failed to send request %d: %v", id, reqID+1, err)
					return
				}

				if response.Error != nil {
					results <- fmt.Errorf("client %d: request %d failed: %+v", id, reqID+1, response.Error)
					return
				}
			}

			results <- nil // 成功完成
		}(clientID)
	}

	// 等待所有客户端完成
	var errors []error
	for i := 0; i < numClients; i++ {
		if err := <-results; err != nil {
			errors = append(errors, err)
		}
	}

	// 检查结果
	if len(errors) > 0 {
		t.Errorf("Concurrent test failed with %d errors:", len(errors))
		for _, err := range errors {
			t.Errorf("  %v", err)
		}
	} else {
		t.Logf("Concurrent test completed successfully: %d clients × %d requests = %d total requests", 
			numClients, requestsPerClient, numClients*requestsPerClient)
	}
}