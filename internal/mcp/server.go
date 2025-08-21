package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/sirupsen/logrus"

	"nucc.com/mcp_srv_mgr/internal/managers"
	"nucc.com/mcp_srv_mgr/pkg/types"
)

type Server struct {
	managers    map[types.ServiceType]types.ServiceManager
	logger      *logrus.Logger
	initialized bool
	logLevel    types.LoggingLevel
}

func NewServer(logger *logrus.Logger) *Server {
	server := &Server{
		managers: make(map[types.ServiceType]types.ServiceManager),
		logger:   logger,
		logLevel: types.LoggingLevelInfo,
	}

	// Initialize available service managers
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

	return server
}

func (s *Server) Start() {
	scanner := bufio.NewScanner(os.Stdin)
	writer := json.NewEncoder(os.Stdout)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var request types.MCPRequest
		if err := json.Unmarshal([]byte(line), &request); err != nil {
			s.sendError(writer, nil, types.ParseError, "Parse error", err)
			continue
		}

		response := s.handleRequest(&request)
		if response != nil {
			writer.Encode(response)
		}
	}
}

func (s *Server) handleRequest(request *types.MCPRequest) *types.MCPResponse {
	switch request.Method {
	case "initialize":
		return s.handleInitialize(request)
	case "initialized":
		return s.handleInitialized(request)
	case "tools/list":
		return s.handleListTools(request)
	case "tools/call":
		return s.handleCallTool(request)
	case "prompts/list":
		return s.handleListPrompts(request)
	case "prompts/get":
		return s.handleGetPrompt(request)
	case "logging/setLevel":
		return s.handleSetLogLevel(request)
	default:
		return s.createErrorResponse(request.ID, types.MethodNotFound, "Method not found", nil)
	}
}

func (s *Server) handleInitialize(request *types.MCPRequest) *types.MCPResponse {
	var params types.InitializeParams
	if request.Params != nil {
		paramsBytes, _ := json.Marshal(request.Params)
		if err := json.Unmarshal(paramsBytes, &params); err != nil {
			return s.createErrorResponse(request.ID, types.InvalidParams, "Invalid params", err)
		}
	}

	result := types.InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities: types.ServerCapabilities{
			Logging: &types.LoggingCapability{},
			Prompts: &types.PromptsCapability{
				ListChanged: false,
			},
			Tools: &types.ToolsCapability{
				ListChanged: false,
			},
		},
		ServerInfo: types.ServerInfo{
			Name:    "Linux Service Manager",
			Version: "1.0.0",
		},
	}

	return s.createSuccessResponse(request.ID, result)
}

func (s *Server) handleInitialized(request *types.MCPRequest) *types.MCPResponse {
	s.initialized = true
	s.logger.Info("MCP Server initialized")
	return nil // No response for notification
}

func (s *Server) handleListTools(request *types.MCPRequest) *types.MCPResponse {
	tools := []types.Tool{
		{
			Name:        "list_services",
			Description: "List all available services from all service managers",
			InputSchema: types.JSONSchema{
				Type: "object",
				Properties: map[string]types.JSONSchema{
					"service_type": {
						Type:        "string",
						Description: "Filter services by type (systemd, sysv, docker)",
						Enum:        []interface{}{"systemd", "sysv", "docker"},
					},
				},
			},
		},
		{
			Name:        "get_service_status",
			Description: "Get detailed status of a specific service",
			InputSchema: types.JSONSchema{
				Type: "object",
				Properties: map[string]types.JSONSchema{
					"service_name": {
						Type:        "string",
						Description: "Name of the service",
					},
					"service_type": {
						Type:        "string",
						Description: "Type of service (systemd, sysv, docker)",
						Enum:        []interface{}{"systemd", "sysv", "docker"},
					},
				},
				Required: []string{"service_name"},
			},
		},
		{
			Name:        "start_service",
			Description: "Start a service",
			InputSchema: types.JSONSchema{
				Type: "object",
				Properties: map[string]types.JSONSchema{
					"service_name": {
						Type:        "string",
						Description: "Name of the service to start",
					},
					"service_type": {
						Type:        "string",
						Description: "Type of service (systemd, sysv, docker)",
						Enum:        []interface{}{"systemd", "sysv", "docker"},
					},
				},
				Required: []string{"service_name"},
			},
		},
		{
			Name:        "stop_service",
			Description: "Stop a service",
			InputSchema: types.JSONSchema{
				Type: "object",
				Properties: map[string]types.JSONSchema{
					"service_name": {
						Type:        "string",
						Description: "Name of the service to stop",
					},
					"service_type": {
						Type:        "string",
						Description: "Type of service (systemd, sysv, docker)",
						Enum:        []interface{}{"systemd", "sysv", "docker"},
					},
				},
				Required: []string{"service_name"},
			},
		},
		{
			Name:        "restart_service",
			Description: "Restart a service",
			InputSchema: types.JSONSchema{
				Type: "object",
				Properties: map[string]types.JSONSchema{
					"service_name": {
						Type:        "string",
						Description: "Name of the service to restart",
					},
					"service_type": {
						Type:        "string",
						Description: "Type of service (systemd, sysv, docker)",
						Enum:        []interface{}{"systemd", "sysv", "docker"},
					},
				},
				Required: []string{"service_name"},
			},
		},
		{
			Name:        "enable_service",
			Description: "Enable a service to start at boot",
			InputSchema: types.JSONSchema{
				Type: "object",
				Properties: map[string]types.JSONSchema{
					"service_name": {
						Type:        "string",
						Description: "Name of the service to enable",
					},
					"service_type": {
						Type:        "string",
						Description: "Type of service (systemd, sysv, docker)",
						Enum:        []interface{}{"systemd", "sysv", "docker"},
					},
				},
				Required: []string{"service_name"},
			},
		},
		{
			Name:        "disable_service",
			Description: "Disable a service from starting at boot",
			InputSchema: types.JSONSchema{
				Type: "object",
				Properties: map[string]types.JSONSchema{
					"service_name": {
						Type:        "string",
						Description: "Name of the service to disable",
					},
					"service_type": {
						Type:        "string",
						Description: "Type of service (systemd, sysv, docker)",
						Enum:        []interface{}{"systemd", "sysv", "docker"},
					},
				},
				Required: []string{"service_name"},
			},
		},
		{
			Name:        "get_docker_logs",
			Description: "Get logs from a Docker container",
			InputSchema: types.JSONSchema{
				Type: "object",
				Properties: map[string]types.JSONSchema{
					"container_name": {
						Type:        "string",
						Description: "Name of the Docker container",
					},
					"lines": {
						Type:        "integer",
						Description: "Number of log lines to retrieve (default: 100)",
					},
				},
				Required: []string{"container_name"},
			},
		},
	}

	result := types.ListToolsResult{Tools: tools}
	return s.createSuccessResponse(request.ID, result)
}

func (s *Server) handleCallTool(request *types.MCPRequest) *types.MCPResponse {
	var params types.CallToolParams
	if request.Params != nil {
		paramsBytes, _ := json.Marshal(request.Params)
		if err := json.Unmarshal(paramsBytes, &params); err != nil {
			return s.createErrorResponse(request.ID, types.InvalidParams, "Invalid params", err)
		}
	}

	switch params.Name {
	case "list_services":
		return s.callListServices(request.ID, params.Arguments)
	case "get_service_status":
		return s.callGetServiceStatus(request.ID, params.Arguments)
	case "start_service":
		return s.callStartService(request.ID, params.Arguments)
	case "stop_service":
		return s.callStopService(request.ID, params.Arguments)
	case "restart_service":
		return s.callRestartService(request.ID, params.Arguments)
	case "enable_service":
		return s.callEnableService(request.ID, params.Arguments)
	case "disable_service":
		return s.callDisableService(request.ID, params.Arguments)
	case "get_docker_logs":
		return s.callGetDockerLogs(request.ID, params.Arguments)
	default:
		return s.createErrorResponse(request.ID, types.MethodNotFound, "Tool not found", nil)
	}
}

func (s *Server) callListServices(id interface{}, args map[string]interface{}) *types.MCPResponse {
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
	result := types.CallToolResult{
		Content: []types.Content{{Type: "text", Text: resultText}},
	}
	return s.createSuccessResponse(id, result)
}

func (s *Server) callGetServiceStatus(id interface{}, args map[string]interface{}) *types.MCPResponse {
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
	result := types.CallToolResult{
		Content: []types.Content{{Type: "text", Text: resultText}},
	}
	return s.createSuccessResponse(id, result)
}

func (s *Server) callStartService(id interface{}, args map[string]interface{}) *types.MCPResponse {
	return s.callServiceOperation(id, args, "start")
}

func (s *Server) callStopService(id interface{}, args map[string]interface{}) *types.MCPResponse {
	return s.callServiceOperation(id, args, "stop")
}

func (s *Server) callRestartService(id interface{}, args map[string]interface{}) *types.MCPResponse {
	return s.callServiceOperation(id, args, "restart")
}

func (s *Server) callEnableService(id interface{}, args map[string]interface{}) *types.MCPResponse {
	return s.callServiceOperation(id, args, "enable")
}

func (s *Server) callDisableService(id interface{}, args map[string]interface{}) *types.MCPResponse {
	return s.callServiceOperation(id, args, "disable")
}

func (s *Server) callServiceOperation(id interface{}, args map[string]interface{}, operation string) *types.MCPResponse {
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

	result := types.CallToolResult{
		Content: []types.Content{{Type: "text", Text: resultText}},
	}
	return s.createSuccessResponse(id, result)
}

func (s *Server) callGetDockerLogs(id interface{}, args map[string]interface{}) *types.MCPResponse {
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
	result := types.CallToolResult{
		Content: []types.Content{{Type: "text", Text: resultText}},
	}
	return s.createSuccessResponse(id, result)
}

func (s *Server) handleListPrompts(request *types.MCPRequest) *types.MCPResponse {
	prompts := []types.Prompt{
		{
			Name:        "service_management_help",
			Description: "Get comprehensive help for managing Linux services",
			Arguments: []types.PromptArgument{
				{
					Name:        "topic",
					Description: "Specific topic to get help for (systemd, sysv, docker, troubleshooting)",
					Required:    false,
				},
			},
		},
		{
			Name:        "service_troubleshooting",
			Description: "Get troubleshooting guidance for service issues",
			Arguments: []types.PromptArgument{
				{
					Name:        "service_name",
					Description: "Name of the service having issues",
					Required:    true,
				},
				{
					Name:        "error_description",
					Description: "Description of the error or issue",
					Required:    false,
				},
			},
		},
	}

	result := types.ListPromptsResult{Prompts: prompts}
	return s.createSuccessResponse(request.ID, result)
}

func (s *Server) handleGetPrompt(request *types.MCPRequest) *types.MCPResponse {
	var params types.GetPromptParams
	if request.Params != nil {
		paramsBytes, _ := json.Marshal(request.Params)
		if err := json.Unmarshal(paramsBytes, &params); err != nil {
			return s.createErrorResponse(request.ID, types.InvalidParams, "Invalid params", err)
		}
	}

	switch params.Name {
	case "service_management_help":
		return s.getServiceManagementHelp(request.ID, params.Arguments)
	case "service_troubleshooting":
		return s.getServiceTroubleshooting(request.ID, params.Arguments)
	default:
		return s.createErrorResponse(request.ID, types.MethodNotFound, "Prompt not found", nil)
	}
}

func (s *Server) getServiceManagementHelp(id interface{}, args map[string]interface{}) *types.MCPResponse {
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

	result := types.GetPromptResult{
		Description: "Service management help and guidance",
		Messages: []types.PromptMessage{
			{
				Role:    "assistant",
				Content: content,
			},
		},
	}
	return s.createSuccessResponse(id, result)
}

func (s *Server) getServiceTroubleshooting(id interface{}, args map[string]interface{}) *types.MCPResponse {
	serviceName, ok := args["service_name"].(string)
	if !ok {
		return s.createErrorResponse(id, types.InvalidParams, "service_name is required", nil)
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

	result := types.GetPromptResult{
		Description: fmt.Sprintf("Troubleshooting guidance for service: %s", serviceName),
		Messages: []types.PromptMessage{
			{
				Role:    "assistant",
				Content: content,
			},
		},
	}
	return s.createSuccessResponse(id, result)
}

func (s *Server) handleSetLogLevel(request *types.MCPRequest) *types.MCPResponse {
	var params types.SetLevelParams
	if request.Params != nil {
		paramsBytes, _ := json.Marshal(request.Params)
		if err := json.Unmarshal(paramsBytes, &params); err != nil {
			return s.createErrorResponse(request.ID, types.InvalidParams, "Invalid params", err)
		}
	}

	s.logLevel = params.Level

	// Update logger level
	switch params.Level {
	case types.LoggingLevelDebug:
		s.logger.SetLevel(logrus.DebugLevel)
	case types.LoggingLevelInfo:
		s.logger.SetLevel(logrus.InfoLevel)
	case types.LoggingLevelWarning:
		s.logger.SetLevel(logrus.WarnLevel)
	case types.LoggingLevelError:
		s.logger.SetLevel(logrus.ErrorLevel)
	}

	return s.createSuccessResponse(request.ID, map[string]interface{}{})
}

// Helper methods

func (s *Server) getServiceManager(serviceName, serviceType string) (types.ServiceManager, error) {
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

func (s *Server) formatServicesOutput(services []types.ServiceInfo) string {
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

func (s *Server) formatServiceInfo(info types.ServiceInfo) string {
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

func (s *Server) createSuccessResponse(id interface{}, result interface{}) *types.MCPResponse {
	return &types.MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
}

func (s *Server) createErrorResponse(id interface{}, code int, message string, data interface{}) *types.MCPResponse {
	return &types.MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &types.MCPError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
}

func (s *Server) createToolErrorResponse(id interface{}, message string) *types.MCPResponse {
	result := types.CallToolResult{
		Content: []types.Content{{Type: "text", Text: fmt.Sprintf("Error: %s", message)}},
		IsError: true,
	}
	return s.createSuccessResponse(id, result)
}

func (s *Server) sendError(writer *json.Encoder, id interface{}, code int, message string, data interface{}) {
	response := s.createErrorResponse(id, code, message, data)
	writer.Encode(response)
}