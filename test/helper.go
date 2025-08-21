package test

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"
)

// ServerInstance 表示一个服务器实例
type ServerInstance struct {
	cmd      *exec.Cmd
	process  *os.Process
	mode     string
	port     int
	stdout   io.ReadCloser
	stderr   io.ReadCloser
	stdin    io.WriteCloser
	started  bool
	mu       sync.Mutex
}

// TestHelper 测试辅助工具
type TestHelper struct {
	serverBinary string
	servers      map[string]*ServerInstance
	mu           sync.Mutex
}

// NewTestHelper 创建测试助手
func NewTestHelper(serverBinary string) *TestHelper {
	return &TestHelper{
		serverBinary: serverBinary,
		servers:      make(map[string]*ServerInstance),
	}
}

// StartServer 启动指定模式的服务器
func (h *TestHelper) StartServer(mode string, port int) (*ServerInstance, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// 检查是否已经启动
	if server, exists := h.servers[mode]; exists {
		return server, nil
	}

	// 创建服务器实例
	var args []string
	if mode != "mcp" {
		args = append(args, fmt.Sprintf("-mode=%s", mode))
	}

	// 创建配置文件（对于需要HTTP端口的模式）
	if mode != "mcp" {
		configFile := fmt.Sprintf("config_%s.yaml", mode)
		err := h.createConfigFile(configFile, port)
		if err != nil {
			return nil, fmt.Errorf("failed to create config file: %v", err)
		}
		args = append(args, fmt.Sprintf("-config=%s", configFile))
	}

	cmd := exec.Command(h.serverBinary, args...)
	
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %v", err)
	}
	
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %v", err)
	}

	server := &ServerInstance{
		cmd:    cmd,
		mode:   mode,
		port:   port,
		stdout: stdout,
		stderr: stderr,
	}

	if mode == "mcp" {
		// MCP stdio 模式需要输入管道
		stdin, err := cmd.StdinPipe()
		if err != nil {
			return nil, fmt.Errorf("failed to create stdin pipe: %v", err)
		}
		server.stdin = stdin
	}

	// 启动服务器
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start server: %v", err)
	}

	server.process = cmd.Process
	server.started = true

	// 等待服务器启动
	if mode != "mcp" {
		if err := h.waitForHTTPServer(port); err != nil {
			server.Stop()
			return nil, fmt.Errorf("server failed to start: %v", err)
		}
	} else {
		// 对于MCP stdio模式，等待一下让服务器初始化
		time.Sleep(500 * time.Millisecond)
	}

	h.servers[mode] = server
	return server, nil
}

// StopServer 停止指定模式的服务器
func (h *TestHelper) StopServer(mode string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	server, exists := h.servers[mode]
	if !exists {
		return fmt.Errorf("server %s not found", mode)
	}

	delete(h.servers, mode)
	return server.Stop()
}

// StopAllServers 停止所有服务器
func (h *TestHelper) StopAllServers() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for mode, server := range h.servers {
		server.Stop()
		delete(h.servers, mode)
	}
}

// Stop 停止服务器实例
func (s *ServerInstance) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started {
		return nil
	}

	if s.process != nil {
		s.process.Signal(os.Interrupt)
		// 等待进程退出
		done := make(chan error, 1)
		go func() {
			done <- s.cmd.Wait()
		}()

		select {
		case <-done:
			// 进程已退出
		case <-time.After(5 * time.Second):
			// 超时，强制杀死进程
			s.process.Kill()
		}
	}

	s.started = false
	return nil
}

// GetStdout 获取服务器标准输出
func (s *ServerInstance) GetStdout() io.ReadCloser {
	return s.stdout
}

// GetStderr 获取服务器错误输出
func (s *ServerInstance) GetStderr() io.ReadCloser {
	return s.stderr
}

// SendMCPMessage 向MCP stdio服务器发送消息
func (s *ServerInstance) SendMCPMessage(message map[string]interface{}) error {
	if s.mode != "mcp" {
		return fmt.Errorf("not an MCP stdio server")
	}

	if s.stdin == nil {
		return fmt.Errorf("stdin not available")
	}

	jsonData, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %v", err)
	}

	_, err = fmt.Fprintf(s.stdin, "%s\n", jsonData)
	return err
}

// ReadMCPResponse 从MCP stdio服务器读取响应
func (s *ServerInstance) ReadMCPResponse() (map[string]interface{}, error) {
	if s.mode != "mcp" {
		return nil, fmt.Errorf("not an MCP stdio server")
	}

	scanner := bufio.NewScanner(s.stdout)
	if scanner.Scan() {
		line := scanner.Text()
		var response map[string]interface{}
		if err := json.Unmarshal([]byte(line), &response); err != nil {
			return nil, fmt.Errorf("failed to unmarshal response: %v", err)
		}
		return response, nil
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	return nil, fmt.Errorf("no response received")
}

// createConfigFile 创建配置文件
func (h *TestHelper) createConfigFile(filename string, port int) error {
	config := fmt.Sprintf(`server:
  host: "127.0.0.1"
  port: %d

log:
  level: "info"
  format: "text"
  output: "stderr"
`, port)

	return os.WriteFile(filename, []byte(config), 0644)
}

// waitForHTTPServer 等待HTTP服务器启动
func (h *TestHelper) waitForHTTPServer(port int) error {
	client := &http.Client{Timeout: 1 * time.Second}
	url := fmt.Sprintf("http://127.0.0.1:%d/health", port)

	for i := 0; i < 30; i++ { // 等待最多30秒
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			return nil
		}
		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("server did not start within timeout")
}

// MCPRequest MCP请求结构
type MCPRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// MCPResponse MCP响应结构
type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

// MCPError MCP错误结构
type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// HTTPClient HTTP客户端助手
type HTTPClient struct {
	BaseURL string
	Client  *http.Client
}

// NewHTTPClient 创建HTTP客户端
func NewHTTPClient(baseURL string) *HTTPClient {
	return &HTTPClient{
		BaseURL: baseURL,
		Client:  &http.Client{Timeout: 10 * time.Second},
	}
}

// Get 发送GET请求
func (c *HTTPClient) Get(path string) (*http.Response, error) {
	return c.Client.Get(c.BaseURL + path)
}

// Post 发送POST请求
func (c *HTTPClient) Post(path string, body io.Reader) (*http.Response, error) {
	return c.Client.Post(c.BaseURL+path, "application/json", body)
}


// LogOutput 日志输出助手
func LogOutput(prefix string, reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		fmt.Printf("[%s] %s\n", prefix, scanner.Text())
	}
}

// WaitForCondition 等待条件满足
func WaitForCondition(condition func() bool, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}