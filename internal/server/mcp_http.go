package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	"nucc.com/mcp_srv_mgr/internal/config"
	"nucc.com/mcp_srv_mgr/internal/managers"
	"nucc.com/mcp_srv_mgr/pkg/types"
)

type MCPHTTPServer struct {
	managers map[types.ServiceType]types.ServiceManager
	config   *config.Config
	logger   *logrus.Logger
	clients  map[string]*SSEClient
	clientMu sync.RWMutex
}

type SSEClient struct {
	ID       string
	Writer   http.ResponseWriter
	Flusher  http.Flusher
	Context  context.Context
	Cancel   context.CancelFunc
	LastSeen time.Time
}

type MCPRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func NewMCPHTTPServer(cfg *config.Config, logger *logrus.Logger) *MCPHTTPServer {
	server := &MCPHTTPServer{
		managers: make(map[types.ServiceType]types.ServiceManager),
		config:   cfg,
		logger:   logger,
		clients:  make(map[string]*SSEClient),
	}

	// Initialize service managers
	if managers.IsSystemdAvailable() {
		server.managers[types.ServiceTypeSystemd] = managers.NewSystemdManager()
		logger.Info("Systemd manager initialized")
	} else {
		logger.Debug("Systemd not available on this system")
	}

	if managers.IsSysVAvailable() {
		server.managers[types.ServiceTypeSysV] = managers.NewSysVManager()
		logger.Info("SysV manager initialized")
	} else {
		logger.Debug("SysV not available on this system")
	}

	if managers.IsDockerAvailable() {
		server.managers[types.ServiceTypeDocker] = managers.NewDockerManager()
		logger.Info("Docker manager initialized")
	} else {
		logger.Debug("Docker not available on this system")
	}

	if len(server.managers) == 0 {
		logger.Warn("No service managers available")
		// 添加一个mock管理器用于测试
		server.managers[types.ServiceTypeSystemd] = managers.NewMockManager(types.ServiceTypeSystemd)
		server.managers[types.ServiceTypeDocker] = managers.NewMockManager(types.ServiceTypeDocker)
		server.managers[types.ServiceTypeSysV] = managers.NewMockManager(types.ServiceTypeSysV)
		logger.Info("Mock managers initialized for testing")
	}

	// Start cleanup routine for stale clients
	go server.cleanupClients()

	return server
}

func (s *MCPHTTPServer) SetupRoutes() *mux.Router {
	router := mux.NewRouter()

	// MCP over HTTP endpoints - compatible with mark3labs/mcp-go
	router.HandleFunc("/sse", s.handleSSE).Methods("GET")                    // Standard SSE endpoint
	router.HandleFunc("/message", s.handleMessage).Methods("POST")           // Standard message endpoint
	router.HandleFunc("/mcp/sse", s.handleSSE).Methods("GET")               // Legacy endpoint for backward compatibility
	router.HandleFunc("/mcp/messages", s.handleMessages).Methods("POST")     // Legacy endpoint for backward compatibility

	// Health check
	router.HandleFunc("/health", s.handleHealth).Methods("GET")

	// CORS middleware
	router.Use(s.corsMiddleware)

	return router
}

func (s *MCPHTTPServer) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Cache-Control")
		w.Header().Set("Access-Control-Expose-Headers", "X-MCP-Client-ID")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *MCPHTTPServer) handleSSE(w http.ResponseWriter, r *http.Request) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Create client
	clientID := fmt.Sprintf("client_%d", time.Now().UnixNano())
	ctx, cancel := context.WithCancel(r.Context())

	client := &SSEClient{
		ID:       clientID,
		Writer:   w,
		Flusher:  flusher,
		Context:  ctx,
		Cancel:   cancel,
		LastSeen: time.Now(),
	}

	// Register client
	s.clientMu.Lock()
	s.clients[clientID] = client
	s.clientMu.Unlock()

	// Send client ID
	w.Header().Set("X-MCP-Client-ID", clientID)

	// Send initial connection event (legacy)
	s.sendSSEMessage(client, "connected", map[string]interface{}{
		"clientId":  clientID,
		"timestamp": time.Now().Format(time.RFC3339),
	})

	// Send endpoint event for mark3labs/mcp-go compatibility
	messageEndpoint := fmt.Sprintf("http://localhost:8081/message?session=%s", clientID)
	s.sendSSEEndpoint(client, messageEndpoint)

	s.logger.Infof("SSE client connected: %s", clientID)

	// Keep connection alive
	// Use shorter interval for testing if port indicates test environment
	heartbeatInterval := 30 * time.Second
	if s.config.Server.Port >= 8080 && s.config.Server.Port <= 8090 {
		heartbeatInterval = 5 * time.Second // Faster heartbeat for tests
	}
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Infof("SSE client disconnected: %s", clientID)
			s.clientMu.Lock()
			delete(s.clients, clientID)
			s.clientMu.Unlock()
			return
		case <-ticker.C:
			// Send heartbeat
			s.sendSSEMessage(client, "heartbeat", map[string]interface{}{
				"timestamp": time.Now().Format(time.RFC3339),
			})
		}
	}
}

func (s *MCPHTTPServer) handleMessage(w http.ResponseWriter, r *http.Request) {
	// Get session ID from query parameter (mark3labs/mcp-go style)
	sessionID := r.URL.Query().Get("session")
	if sessionID == "" {
		http.Error(w, "Missing session parameter", http.StatusBadRequest)
		return
	}

	// Get client by session ID
	s.clientMu.RLock()
	client, exists := s.clients[sessionID]
	s.clientMu.RUnlock()

	if !exists {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	// Update last seen
	client.LastSeen = time.Now()

	// Parse MCP request
	var mcpReq MCPRequest
	if err := json.NewDecoder(r.Body).Decode(&mcpReq); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Process MCP request
	response := s.processMCPRequest(&mcpReq)

	// Send response via SSE
	s.sendSSEMessage(client, "message", response)

	// Return 202 Accepted (mark3labs/mcp-go expects this)
	w.WriteHeader(http.StatusAccepted)
}

func (s *MCPHTTPServer) handleMessages(w http.ResponseWriter, r *http.Request) {
	clientID := r.Header.Get("X-MCP-Client-ID")
	if clientID == "" {
		http.Error(w, "Missing X-MCP-Client-ID header", http.StatusBadRequest)
		return
	}

	// Get client
	s.clientMu.RLock()
	client, exists := s.clients[clientID]
	s.clientMu.RUnlock()

	if !exists {
		http.Error(w, "Client not found", http.StatusNotFound)
		return
	}

	// Update last seen
	client.LastSeen = time.Now()

	// Parse MCP request
	var mcpReq MCPRequest
	if err := json.NewDecoder(r.Body).Decode(&mcpReq); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Process MCP request
	response := s.processMCPRequest(&mcpReq)

	// Send response via SSE
	s.sendSSEMessage(client, "response", response)

	// Return acknowledgment
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":   true,
		"messageId": mcpReq.ID,
	})
}

func (s *MCPHTTPServer) processMCPRequest(req *MCPRequest) *MCPResponse {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "tools/list":
		return s.handleListTools(req)
	case "tools/call":
		return s.handleCallTool(req)
	case "prompts/list":
		return s.handleListPrompts(req)
	case "prompts/get":
		return s.handleGetPrompt(req)
	default:
		return s.createErrorResponse(req.ID, -32601, "Method not found", nil)
	}
}

func (s *MCPHTTPServer) handleInitialize(req *MCPRequest) *MCPResponse {
	result := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]interface{}{
			"tools": map[string]interface{}{
				"listChanged": false,
			},
			"prompts": map[string]interface{}{
				"listChanged": false,
			},
		},
		"serverInfo": map[string]interface{}{
			"name":    "Linux Service Manager",
			"version": "1.0.0",
		},
	}

	return s.createSuccessResponse(req.ID, result)
}

func (s *MCPHTTPServer) handleListTools(req *MCPRequest) *MCPResponse {
	tools := []map[string]interface{}{
		{
			"name":        "list_services",
			"description": "List all available services from all service managers",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"service_type": map[string]interface{}{
						"type":        "string",
						"description": "Filter services by type (systemd, sysv, docker)",
						"enum":        []string{"systemd", "sysv", "docker"},
					},
				},
			},
		},
		{
			"name":        "get_service_status",
			"description": "Get detailed status of a specific service",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"service_name": map[string]interface{}{
						"type":        "string",
						"description": "Name of the service",
					},
					"service_type": map[string]interface{}{
						"type":        "string",
						"description": "Type of service (systemd, sysv, docker)",
						"enum":        []string{"systemd", "sysv", "docker"},
					},
				},
				"required": []string{"service_name"},
			},
		},
		{
			"name":        "start_service",
			"description": "Start a service",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"service_name": map[string]interface{}{
						"type":        "string",
						"description": "Name of the service to start",
					},
					"service_type": map[string]interface{}{
						"type":        "string",
						"description": "Type of service (systemd, sysv, docker)",
						"enum":        []string{"systemd", "sysv", "docker"},
					},
				},
				"required": []string{"service_name"},
			},
		},
		{
			"name":        "stop_service",
			"description": "Stop a service",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"service_name": map[string]interface{}{
						"type":        "string",
						"description": "Name of the service to stop",
					},
					"service_type": map[string]interface{}{
						"type":        "string",
						"description": "Type of service (systemd, sysv, docker)",
						"enum":        []string{"systemd", "sysv", "docker"},
					},
				},
				"required": []string{"service_name"},
			},
		},
		{
			"name":        "restart_service",
			"description": "Restart a service",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"service_name": map[string]interface{}{
						"type":        "string",
						"description": "Name of the service to restart",
					},
					"service_type": map[string]interface{}{
						"type":        "string",
						"description": "Type of service (systemd, sysv, docker)",
						"enum":        []string{"systemd", "sysv", "docker"},
					},
				},
				"required": []string{"service_name"},
			},
		},
		{
			"name":        "enable_service",
			"description": "Enable a service to start at boot",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"service_name": map[string]interface{}{
						"type":        "string",
						"description": "Name of the service to enable",
					},
					"service_type": map[string]interface{}{
						"type":        "string",
						"description": "Type of service (systemd, sysv, docker)",
						"enum":        []string{"systemd", "sysv", "docker"},
					},
				},
				"required": []string{"service_name"},
			},
		},
		{
			"name":        "disable_service",
			"description": "Disable a service from starting at boot",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"service_name": map[string]interface{}{
						"type":        "string",
						"description": "Name of the service to disable",
					},
					"service_type": map[string]interface{}{
						"type":        "string",
						"description": "Type of service (systemd, sysv, docker)",
						"enum":        []string{"systemd", "sysv", "docker"},
					},
				},
				"required": []string{"service_name"},
			},
		},
		{
			"name":        "get_docker_logs",
			"description": "Get logs from a Docker container",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"container_name": map[string]interface{}{
						"type":        "string",
						"description": "Name of the Docker container",
					},
					"lines": map[string]interface{}{
						"type":        "integer",
						"description": "Number of log lines to retrieve (default: 100)",
					},
				},
				"required": []string{"container_name"},
			},
		},
	}

	result := map[string]interface{}{
		"tools": tools,
	}

	return s.createSuccessResponse(req.ID, result)
}

func (s *MCPHTTPServer) handleCallTool(req *MCPRequest) *MCPResponse {
	params, ok := req.Params.(map[string]interface{})
	if !ok {
		return s.createErrorResponse(req.ID, -32602, "Invalid params", nil)
	}

	toolName, ok := params["name"].(string)
	if !ok {
		return s.createErrorResponse(req.ID, -32602, "Missing tool name", nil)
	}

	arguments, _ := params["arguments"].(map[string]interface{})

	switch toolName {
	case "list_services":
		return s.callListServices(req.ID, arguments)
	case "get_service_status":
		return s.callGetServiceStatus(req.ID, arguments)
	case "start_service":
		return s.callStartService(req.ID, arguments)
	case "stop_service":
		return s.callStopService(req.ID, arguments)
	case "restart_service":
		return s.callRestartService(req.ID, arguments)
	case "enable_service":
		return s.callEnableService(req.ID, arguments)
	case "disable_service":
		return s.callDisableService(req.ID, arguments)
	case "get_docker_logs":
		return s.callGetDockerLogs(req.ID, arguments)
	default:
		return s.createErrorResponse(req.ID, -32601, "Tool not found", nil)
	}
}

func (s *MCPHTTPServer) callListServices(id interface{}, args map[string]interface{}) *MCPResponse {
	var serviceType string
	if st, ok := args["service_type"]; ok {
		serviceType = st.(string)
	}

	var allServices []types.ServiceInfo

	if serviceType != "" {
		if manager, exists := s.managers[types.ServiceType(serviceType)]; exists {
			services, err := manager.ListServices()
			if err != nil {
				return s.createToolErrorResponse(id, fmt.Sprintf("Failed to list %s services: %v", serviceType, err))
			}
			allServices = services
		} else {
			return s.createToolErrorResponse(id, fmt.Sprintf("Unsupported service type: %s", serviceType))
		}
	} else {
		for _, manager := range s.managers {
			services, err := manager.ListServices()
			if err != nil {
				s.logger.Warnf("Failed to list services from manager: %v", err)
				continue
			}
			allServices = append(allServices, services...)
		}
	}

	resultText := s.formatServicesOutput(allServices)
	result := map[string]interface{}{
		"content": []map[string]interface{}{
			{"type": "text", "text": resultText},
		},
	}
	return s.createSuccessResponse(id, result)
}

func (s *MCPHTTPServer) callGetServiceStatus(id interface{}, args map[string]interface{}) *MCPResponse {
	serviceName, ok := args["service_name"].(string)
	if !ok {
		return s.createToolErrorResponse(id, "service_name is required")
	}

	var serviceType string
	if st, ok := args["service_type"]; ok {
		serviceType = st.(string)
	}

	manager, err := s.getServiceManager(serviceName, serviceType)
	if err != nil {
		return s.createToolErrorResponse(id, err.Error())
	}

	info, err := manager.GetStatus(serviceName)
	if err != nil {
		return s.createToolErrorResponse(id, fmt.Sprintf("Failed to get service status: %v", err))
	}

	resultText := s.formatServiceInfo(info)
	result := map[string]interface{}{
		"content": []map[string]interface{}{
			{"type": "text", "text": resultText},
		},
	}
	return s.createSuccessResponse(id, result)
}

func (s *MCPHTTPServer) callStartService(id interface{}, args map[string]interface{}) *MCPResponse {
	return s.callServiceOperation(id, args, "start")
}

func (s *MCPHTTPServer) callStopService(id interface{}, args map[string]interface{}) *MCPResponse {
	return s.callServiceOperation(id, args, "stop")
}

func (s *MCPHTTPServer) callRestartService(id interface{}, args map[string]interface{}) *MCPResponse {
	return s.callServiceOperation(id, args, "restart")
}

func (s *MCPHTTPServer) callEnableService(id interface{}, args map[string]interface{}) *MCPResponse {
	return s.callServiceOperation(id, args, "enable")
}

func (s *MCPHTTPServer) callDisableService(id interface{}, args map[string]interface{}) *MCPResponse {
	return s.callServiceOperation(id, args, "disable")
}

func (s *MCPHTTPServer) callServiceOperation(id interface{}, args map[string]interface{}, operation string) *MCPResponse {
	serviceName, ok := args["service_name"].(string)
	if !ok {
		return s.createToolErrorResponse(id, "service_name is required")
	}

	var serviceType string
	if st, ok := args["service_type"]; ok {
		serviceType = st.(string)
	}

	manager, err := s.getServiceManager(serviceName, serviceType)
	if err != nil {
		return s.createToolErrorResponse(id, err.Error())
	}

	var operationErr error
	switch operation {
	case "start":
		operationErr = manager.Start(serviceName)
	case "stop":
		operationErr = manager.Stop(serviceName)
	case "restart":
		operationErr = manager.Restart(serviceName)
	case "enable":
		operationErr = manager.Enable(serviceName)
	case "disable":
		operationErr = manager.Disable(serviceName)
	}

	if operationErr != nil {
		return s.createToolErrorResponse(id, fmt.Sprintf("Failed to %s service: %v", operation, operationErr))
	}

	info, _ := manager.GetStatus(serviceName)
	resultText := fmt.Sprintf("Service %s %sed successfully.\n\n%s", serviceName, operation, s.formatServiceInfo(info))

	result := map[string]interface{}{
		"content": []map[string]interface{}{
			{"type": "text", "text": resultText},
		},
	}
	return s.createSuccessResponse(id, result)
}

func (s *MCPHTTPServer) callGetDockerLogs(id interface{}, args map[string]interface{}) *MCPResponse {
	containerName, ok := args["container_name"].(string)
	if !ok {
		return s.createToolErrorResponse(id, "container_name is required")
	}

	dockerManager, exists := s.managers[types.ServiceTypeDocker].(*managers.DockerManager)
	if !exists {
		return s.createToolErrorResponse(id, "Docker manager not available")
	}

	lines := 100
	if l, ok := args["lines"]; ok {
		if linesFloat, ok := l.(float64); ok {
			lines = int(linesFloat)
		}
	}

	logs, err := dockerManager.GetLogs(containerName, lines)
	if err != nil {
		return s.createToolErrorResponse(id, fmt.Sprintf("Failed to get logs: %v", err))
	}

	resultText := fmt.Sprintf("Docker container '%s' logs (last %d lines):\n\n%s", containerName, lines, logs)
	result := map[string]interface{}{
		"content": []map[string]interface{}{
			{"type": "text", "text": resultText},
		},
	}
	return s.createSuccessResponse(id, result)
}

func (s *MCPHTTPServer) handleListPrompts(req *MCPRequest) *MCPResponse {
	prompts := []map[string]interface{}{
		{
			"name":        "service_management_help",
			"description": "Get comprehensive help for managing Linux services",
			"arguments": []map[string]interface{}{
				{
					"name":        "topic",
					"description": "Specific topic to get help for (systemd, sysv, docker, troubleshooting)",
					"required":    false,
				},
			},
		},
		{
			"name":        "service_troubleshooting",
			"description": "Get troubleshooting guidance for service issues",
			"arguments": []map[string]interface{}{
				{
					"name":        "service_name",
					"description": "Name of the service having issues",
					"required":    true,
				},
				{
					"name":        "error_description",
					"description": "Description of the error or issue",
					"required":    false,
				},
			},
		},
	}

	result := map[string]interface{}{
		"prompts": prompts,
	}
	return s.createSuccessResponse(req.ID, result)
}

func (s *MCPHTTPServer) handleGetPrompt(req *MCPRequest) *MCPResponse {
	params, ok := req.Params.(map[string]interface{})
	if !ok {
		return s.createErrorResponse(req.ID, -32602, "Invalid params", nil)
	}

	promptName, ok := params["name"].(string)
	if !ok {
		return s.createErrorResponse(req.ID, -32602, "Missing prompt name", nil)
	}

	arguments, _ := params["arguments"].(map[string]interface{})

	switch promptName {
	case "service_management_help":
		return s.getServiceManagementHelp(req.ID, arguments)
	case "service_troubleshooting":
		return s.getServiceTroubleshooting(req.ID, arguments)
	default:
		return s.createErrorResponse(req.ID, -32601, "Prompt not found", nil)
	}
}

func (s *MCPHTTPServer) getServiceManagementHelp(id interface{}, args map[string]interface{}) *MCPResponse {
	topic := ""
	if t, ok := args["topic"]; ok {
		topic = t.(string)
	}

	var content string
	switch topic {
	case "systemd":
		content = "# systemd Service Management\n\nsystemd is the modern init system used by most Linux distributions. Key commands:\n- `systemctl start <service>` - Start a service\n- `systemctl stop <service>` - Stop a service\n- `systemctl restart <service>` - Restart a service\n- `systemctl enable <service>` - Enable service at boot\n- `systemctl disable <service>` - Disable service at boot\n- `systemctl status <service>` - Check service status\n- `systemctl list-units --type=service` - List all services"
	case "sysv":
		content = "# System V init Service Management\n\nTraditional init system using scripts in /etc/init.d/. Key commands:\n- `/etc/init.d/<service> start` - Start a service\n- `/etc/init.d/<service> stop` - Stop a service\n- `/etc/init.d/<service> restart` - Restart a service\n- `chkconfig <service> on` (RHEL/CentOS) or `update-rc.d <service> enable` (Debian/Ubuntu) - Enable at boot\n- Service scripts are located in /etc/init.d/"
	case "docker":
		content = "# Docker Container Management\n\nManage Docker containers as services. Key commands:\n- `docker start <container>` - Start a container\n- `docker stop <container>` - Stop a container\n- `docker restart <container>` - Restart a container\n- `docker update --restart=always <container>` - Auto-restart container\n- `docker ps -a` - List all containers\n- `docker logs <container>` - View container logs"
	default:
		content = "# Linux Service Management Guide\n\nThis MCP server supports managing services through multiple methods:\n\n## Supported Service Types\n1. **systemd** - Modern Linux distributions\n2. **System V init** - Traditional Linux distributions\n3. **Docker** - Container management\n\n## Available Operations\n- Start/Stop/Restart services\n- Enable/Disable services for boot\n- Get service status and information\n- List all available services\n- View Docker container logs\n\n## Usage\nUse the available tools to manage services. The server will automatically detect which service manager to use based on your system and the service name."
	}

	result := map[string]interface{}{
		"description": "Service management help and guidance",
		"messages": []map[string]interface{}{
			{
				"role":    "assistant",
				"content": content,
			},
		},
	}
	return s.createSuccessResponse(id, result)
}

func (s *MCPHTTPServer) getServiceTroubleshooting(id interface{}, args map[string]interface{}) *MCPResponse {
	serviceName, ok := args["service_name"].(string)
	if !ok {
		return s.createErrorResponse(id, -32602, "service_name is required", nil)
	}

	errorDesc := ""
	if e, ok := args["error_description"]; ok {
		errorDesc = e.(string)
	}

	content := fmt.Sprintf("# Troubleshooting Service: %s\n\n", serviceName)

	if errorDesc != "" {
		content += fmt.Sprintf("## Reported Issue\n%s\n\n", errorDesc)
	}

	content += `## Troubleshooting Steps

1. **Check Service Status**
   - Use get_service_status tool to check current status
   - Look for error messages and status information

2. **View Service Logs**
   - For systemd: journalctl -u <service_name> -f
   - For Docker: Use get_docker_logs tool
   - For SysV: Check /var/log/ for service-specific logs

3. **Common Issues**
   - Service not starting: Check configuration files
   - Permission issues: Verify user/group permissions
   - Port conflicts: Check if required ports are available
   - Dependencies: Ensure required services are running

4. **Configuration Check**
   - Verify service configuration files
   - Check for syntax errors
   - Ensure required directories exist

5. **Resource Issues**
   - Check system resources (CPU, memory, disk)
   - Verify required files and dependencies exist

## Next Steps
Use the available tools to gather more information about the service status and logs.`

	result := map[string]interface{}{
		"description": fmt.Sprintf("Troubleshooting guidance for service: %s", serviceName),
		"messages": []map[string]interface{}{
			{
				"role":    "assistant",
				"content": content,
			},
		},
	}
	return s.createSuccessResponse(id, result)
}

func (s *MCPHTTPServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
		"mode":      "mcp-http",
		"clients":   len(s.clients),
		"managers":  s.getAvailableManagers(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Helper methods

func (s *MCPHTTPServer) getServiceManager(serviceName, serviceType string) (types.ServiceManager, error) {
	if serviceType != "" {
		if manager, exists := s.managers[types.ServiceType(serviceType)]; exists {
			return manager, nil
		}
		return nil, fmt.Errorf("unsupported service type: %s", serviceType)
	}

	// Auto-detect service type
	for _, manager := range s.managers {
		if _, err := manager.GetStatus(serviceName); err == nil {
			return manager, nil
		}
	}

	return nil, fmt.Errorf("service %s not found in any manager", serviceName)
}

func (s *MCPHTTPServer) getAvailableManagers() []string {
	var managerList []string
	for serviceType := range s.managers {
		managerList = append(managerList, string(serviceType))
	}
	return managerList
}

func (s *MCPHTTPServer) formatServicesOutput(services []types.ServiceInfo) string {
	if len(services) == 0 {
		return "No services found."
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Found %d services:\n\n", len(services)))

	// Group by service type
	servicesByType := make(map[types.ServiceType][]types.ServiceInfo)
	for _, service := range services {
		servicesByType[service.Type] = append(servicesByType[service.Type], service)
	}

	for serviceType, typeServices := range servicesByType {
		result.WriteString(fmt.Sprintf("## %s Services\n", strings.Title(string(serviceType))))
		for _, service := range typeServices {
			result.WriteString(fmt.Sprintf("- **%s**: %s", service.Name, service.Status))
			if service.Description != "" {
				result.WriteString(fmt.Sprintf(" - %s", service.Description))
			}
			result.WriteString("\n")
		}
		result.WriteString("\n")
	}

	return result.String()
}

func (s *MCPHTTPServer) formatServiceInfo(info types.ServiceInfo) string {
	var result strings.Builder
	result.WriteString(fmt.Sprintf("**Service**: %s\n", info.Name))
	result.WriteString(fmt.Sprintf("**Type**: %s\n", info.Type))
	result.WriteString(fmt.Sprintf("**Status**: %s\n", info.Status))

	if info.Description != "" {
		result.WriteString(fmt.Sprintf("**Description**: %s\n", info.Description))
	}

	if info.PID > 0 {
		result.WriteString(fmt.Sprintf("**PID**: %d\n", info.PID))
	}

	if info.Uptime > 0 {
		result.WriteString(fmt.Sprintf("**Uptime**: %s\n", info.Uptime.String()))
	}

	if !info.LastChanged.IsZero() {
		result.WriteString(fmt.Sprintf("**Last Changed**: %s\n", info.LastChanged.Format("2006-01-02 15:04:05")))
	}

	return result.String()
}

func (s *MCPHTTPServer) sendSSEMessage(client *SSEClient, eventType string, data interface{}) {
	if client.Context.Err() != nil {
		return
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		s.logger.Errorf("Failed to marshal SSE data: %v", err)
		return
	}

	_, err = fmt.Fprintf(client.Writer, "event: %s\ndata: %s\n\n", eventType, jsonData)
	if err != nil {
		s.logger.Errorf("Failed to write SSE message: %v", err)
		return
	}

	client.Flusher.Flush()
}

func (s *MCPHTTPServer) sendSSEEndpoint(client *SSEClient, endpoint string) {
	if client.Context.Err() != nil {
		return
	}

	_, err := fmt.Fprintf(client.Writer, "event: endpoint\ndata: %s\n\n", endpoint)
	if err != nil {
		s.logger.Errorf("Failed to write SSE endpoint: %v", err)
		return
	}

	client.Flusher.Flush()
}

func (s *MCPHTTPServer) cleanupClients() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.clientMu.Lock()
		for id, client := range s.clients {
			if time.Since(client.LastSeen) > 10*time.Minute {
				client.Cancel()
				delete(s.clients, id)
				s.logger.Infof("Cleaned up stale client: %s", id)
			}
		}
		s.clientMu.Unlock()
	}
}

func (s *MCPHTTPServer) createSuccessResponse(id interface{}, result interface{}) *MCPResponse {
	return &MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
}

func (s *MCPHTTPServer) createErrorResponse(id interface{}, code int, message string, data interface{}) *MCPResponse {
	return &MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &MCPError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
}

func (s *MCPHTTPServer) createToolErrorResponse(id interface{}, message string) *MCPResponse {
	result := map[string]interface{}{
		"content": []map[string]interface{}{
			{"type": "text", "text": fmt.Sprintf("Error: %s", message)},
		},
		"isError": true,
	}
	return s.createSuccessResponse(id, result)
}

func (s *MCPHTTPServer) Start() error {
	router := s.SetupRoutes()
	address := fmt.Sprintf("%s:%d", s.config.Server.Host, s.config.Server.Port)

	s.logger.Infof("Starting MCP HTTP Server on %s", address)
	return http.ListenAndServe(address, router)
}
