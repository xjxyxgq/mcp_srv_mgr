package server

import (
	"bufio"
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

type MCPStreamableServer struct {
	managers map[types.ServiceType]types.ServiceManager
	config   *config.Config
	logger   *logrus.Logger
	sessions map[string]*StreamableSession
	sessMu   sync.RWMutex
}

type StreamableSession struct {
	ID         string
	Writer     http.ResponseWriter
	Flusher    http.Flusher
	Context    context.Context
	Cancel     context.CancelFunc
	LastSeen   time.Time
	Requests   chan *StreamableRequest
	Responses  chan *StreamableResponse
	initialized bool
}

type StreamableRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type StreamableResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *StreamableError `json:"error,omitempty"`
}

type StreamableError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func NewMCPStreamableServer(cfg *config.Config, logger *logrus.Logger) *MCPStreamableServer {
	server := &MCPStreamableServer{
		managers: make(map[types.ServiceType]types.ServiceManager),
		config:   cfg,
		logger:   logger,
		sessions: make(map[string]*StreamableSession),
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

	// Start cleanup routine for stale sessions
	go server.cleanupSessions()

	return server
}

func (s *MCPStreamableServer) SetupRoutes() *mux.Router {
	router := mux.NewRouter()

	// MCP Streamable HTTP endpoints
	router.HandleFunc("/mcp/stream", s.handleStream).Methods("POST")
	router.HandleFunc("/mcp/stream/{sessionId}", s.handleSessionStream).Methods("GET")
	
	// Health check
	router.HandleFunc("/health", s.handleHealth).Methods("GET")

	// CORS middleware
	router.Use(s.corsMiddleware)

	return router
}

func (s *MCPStreamableServer) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Cache-Control")
		w.Header().Set("Access-Control-Expose-Headers", "X-MCP-Session-ID")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *MCPStreamableServer) handleStream(w http.ResponseWriter, r *http.Request) {
	// Check if this is a streaming request (bidirectional)
	connection := strings.ToLower(r.Header.Get("Connection"))
	upgrade := strings.ToLower(r.Header.Get("Upgrade"))
	
	if connection == "upgrade" || upgrade == "mcp-stream" || upgrade == "websocket" {
		// Handle streaming upgrade (bidirectional)
		s.handleStreamingUpgrade(w, r)
		return
	}

	// Handle single request-response
	s.handleSingleRequest(w, r)
}

func (s *MCPStreamableServer) handleStreamingUpgrade(w http.ResponseWriter, r *http.Request) {
	// Set streaming headers
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "chunked")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Create session
	sessionID := fmt.Sprintf("stream_%d", time.Now().UnixNano())
	ctx, cancel := context.WithCancel(r.Context())

	session := &StreamableSession{
		ID:        sessionID,
		Writer:    w,
		Flusher:   flusher,
		Context:   ctx,
		Cancel:    cancel,
		LastSeen:  time.Now(),
		Requests:  make(chan *StreamableRequest, 10),
		Responses: make(chan *StreamableResponse, 10),
	}

	// Register session
	s.sessMu.Lock()
	s.sessions[sessionID] = session
	s.sessMu.Unlock()

	// Send session ID in header
	w.Header().Set("X-MCP-Session-ID", sessionID)
	
	// Send status 200 OK to complete the upgrade handshake
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	s.logger.Infof("Streamable session started: %s", sessionID)

	// Start processing goroutine
	go s.processSession(session)

	// Read from request body (streaming input)
	scanner := bufio.NewScanner(r.Body)
	for scanner.Scan() {
		if session.Context.Err() != nil {
			break
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var req StreamableRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			s.logger.Errorf("Failed to parse request: %v", err)
			continue
		}

		session.LastSeen = time.Now()

		select {
		case session.Requests <- &req:
		case <-session.Context.Done():
			return
		default:
			s.logger.Warn("Request channel full, dropping request")
		}
	}

	// Cleanup
	s.sessMu.Lock()
	delete(s.sessions, sessionID)
	s.sessMu.Unlock()
	cancel()
	s.logger.Infof("Streamable session ended: %s", sessionID)
}

func (s *MCPStreamableServer) handleSingleRequest(w http.ResponseWriter, r *http.Request) {
	var req StreamableRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Process request
	response := s.processMCPRequest(&req)

	// Send response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *MCPStreamableServer) handleSessionStream(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionID := vars["sessionId"]

	// Get session
	s.sessMu.RLock()
	session, exists := s.sessions[sessionID]
	s.sessMu.RUnlock()

	if !exists {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	// Set streaming headers
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Stream responses
	for {
		select {
		case response := <-session.Responses:
			jsonData, err := json.Marshal(response)
			if err != nil {
				s.logger.Errorf("Failed to marshal response: %v", err)
				continue
			}
			
			if _, err := w.Write(jsonData); err != nil {
				s.logger.Errorf("Failed to write response: %v", err)
				return
			}
			if _, err := w.Write([]byte("\n")); err != nil {
				s.logger.Errorf("Failed to write newline: %v", err)
				return
			}
			flusher.Flush()

		case <-session.Context.Done():
			return
		case <-r.Context().Done():
			return
		}
	}
}

func (s *MCPStreamableServer) processSession(session *StreamableSession) {
	for {
		select {
		case req := <-session.Requests:
			response := s.processMCPRequest(req)
			
			select {
			case session.Responses <- response:
			case <-session.Context.Done():
				return
			default:
				s.logger.Warn("Response channel full, dropping response")
			}

		case <-session.Context.Done():
			return
		}
	}
}

func (s *MCPStreamableServer) processMCPRequest(req *StreamableRequest) *StreamableResponse {
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

func (s *MCPStreamableServer) handleInitialize(req *StreamableRequest) *StreamableResponse {
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

func (s *MCPStreamableServer) handleListTools(req *StreamableRequest) *StreamableResponse {
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

func (s *MCPStreamableServer) handleCallTool(req *StreamableRequest) *StreamableResponse {
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

func (s *MCPStreamableServer) callListServices(id interface{}, args map[string]interface{}) *StreamableResponse {
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

func (s *MCPStreamableServer) callGetServiceStatus(id interface{}, args map[string]interface{}) *StreamableResponse {
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

func (s *MCPStreamableServer) callStartService(id interface{}, args map[string]interface{}) *StreamableResponse {
	return s.callServiceOperation(id, args, "start")
}

func (s *MCPStreamableServer) callStopService(id interface{}, args map[string]interface{}) *StreamableResponse {
	return s.callServiceOperation(id, args, "stop")
}

func (s *MCPStreamableServer) callRestartService(id interface{}, args map[string]interface{}) *StreamableResponse {
	return s.callServiceOperation(id, args, "restart")
}

func (s *MCPStreamableServer) callEnableService(id interface{}, args map[string]interface{}) *StreamableResponse {
	return s.callServiceOperation(id, args, "enable")
}

func (s *MCPStreamableServer) callDisableService(id interface{}, args map[string]interface{}) *StreamableResponse {
	return s.callServiceOperation(id, args, "disable")
}

func (s *MCPStreamableServer) callServiceOperation(id interface{}, args map[string]interface{}, operation string) *StreamableResponse {
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

func (s *MCPStreamableServer) callGetDockerLogs(id interface{}, args map[string]interface{}) *StreamableResponse {
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

func (s *MCPStreamableServer) handleListPrompts(req *StreamableRequest) *StreamableResponse {
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

func (s *MCPStreamableServer) handleGetPrompt(req *StreamableRequest) *StreamableResponse {
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

func (s *MCPStreamableServer) getServiceManagementHelp(id interface{}, args map[string]interface{}) *StreamableResponse {
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

func (s *MCPStreamableServer) getServiceTroubleshooting(id interface{}, args map[string]interface{}) *StreamableResponse {
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

func (s *MCPStreamableServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
		"mode":      "mcp-streamable",
		"sessions":  len(s.sessions),
		"managers":  s.getAvailableManagers(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Helper methods

func (s *MCPStreamableServer) getServiceManager(serviceName, serviceType string) (types.ServiceManager, error) {
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

func (s *MCPStreamableServer) getAvailableManagers() []string {
	var managerList []string
	for serviceType := range s.managers {
		managerList = append(managerList, string(serviceType))
	}
	return managerList
}

func (s *MCPStreamableServer) formatServicesOutput(services []types.ServiceInfo) string {
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

func (s *MCPStreamableServer) formatServiceInfo(info types.ServiceInfo) string {
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

func (s *MCPStreamableServer) cleanupSessions() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.sessMu.Lock()
		for id, session := range s.sessions {
			if time.Since(session.LastSeen) > 10*time.Minute {
				session.Cancel()
				delete(s.sessions, id)
				s.logger.Infof("Cleaned up stale session: %s", id)
			}
		}
		s.sessMu.Unlock()
	}
}

func (s *MCPStreamableServer) createSuccessResponse(id interface{}, result interface{}) *StreamableResponse {
	return &StreamableResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
}

func (s *MCPStreamableServer) createErrorResponse(id interface{}, code int, message string, data interface{}) *StreamableResponse {
	return &StreamableResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &StreamableError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
}

func (s *MCPStreamableServer) createToolErrorResponse(id interface{}, message string) *StreamableResponse {
	result := map[string]interface{}{
		"content": []map[string]interface{}{
			{"type": "text", "text": fmt.Sprintf("Error: %s", message)},
		},
		"isError": true,
	}
	return s.createSuccessResponse(id, result)
}

func (s *MCPStreamableServer) Start() error {
	router := s.SetupRoutes()
	address := fmt.Sprintf("%s:%d", s.config.Server.Host, s.config.Server.Port)

	s.logger.Infof("Starting MCP Streamable Server on %s", address)
	return http.ListenAndServe(address, router)
}