package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	"nucc.com/mcp_srv_mgr/internal/config"
	"nucc.com/mcp_srv_mgr/internal/managers"
	"nucc.com/mcp_srv_mgr/pkg/types"
	"nucc.com/mcp_srv_mgr/pkg/utils"
)

type HTTPServer struct {
	managers map[types.ServiceType]types.ServiceManager
	config   *config.Config
	logger   *logrus.Logger
}

func NewHTTPServer(cfg *config.Config, logger *logrus.Logger) *HTTPServer {
	server := &HTTPServer{
		managers: make(map[types.ServiceType]types.ServiceManager),
		config:   cfg,
		logger:   logger,
	}

	// Initialize available service managers
	if managers.IsSystemdAvailable() {
		server.managers[types.ServiceTypeSystemd] = managers.NewSystemdManager()
		logger.Info("Systemd manager initialized")
	}

	if managers.IsSysVAvailable() {
		server.managers[types.ServiceTypeSysV] = managers.NewSysVManager()
		logger.Info("SysV manager initialized")
	}

	if managers.IsDockerAvailable() {
		server.managers[types.ServiceTypeDocker] = managers.NewDockerManager()
		logger.Info("Docker manager initialized")
	}

	if len(server.managers) == 0 {
		logger.Warn("No service managers available")
	}

	return server
}

func (s *HTTPServer) SetupRoutes() *mux.Router {
	router := mux.NewRouter()

	// Service management endpoints
	router.HandleFunc("/services", s.handleListServices).Methods("GET")
	router.HandleFunc("/services/{name}/status", s.handleGetStatus).Methods("GET")
	router.HandleFunc("/services/{name}/start", s.handleStartService).Methods("POST")
	router.HandleFunc("/services/{name}/stop", s.handleStopService).Methods("POST")
	router.HandleFunc("/services/{name}/restart", s.handleRestartService).Methods("POST")
	router.HandleFunc("/services/{name}/enable", s.handleEnableService).Methods("POST")
	router.HandleFunc("/services/{name}/disable", s.handleDisableService).Methods("POST")

	// Generic service action endpoint
	router.HandleFunc("/services/action", s.handleServiceAction).Methods("POST")

	// Docker-specific endpoints
	router.HandleFunc("/docker/{name}/logs", s.handleDockerLogs).Methods("GET")
	router.HandleFunc("/docker/{name}/stats", s.handleDockerStats).Methods("GET")
	router.HandleFunc("/docker/{name}/remove", s.handleDockerRemove).Methods("DELETE")
	router.HandleFunc("/docker/create", s.handleDockerCreate).Methods("POST")

	// Health check endpoint
	router.HandleFunc("/health", s.handleHealth).Methods("GET")

	// Info endpoint
	router.HandleFunc("/info", s.handleInfo).Methods("GET")

	return router
}

func (s *HTTPServer) handleListServices(w http.ResponseWriter, r *http.Request) {
	serviceType := r.URL.Query().Get("type")
	var allServices []types.ServiceInfo

	if serviceType != "" {
		// List services for specific type
		if manager, exists := s.managers[types.ServiceType(serviceType)]; exists {
			services, err := manager.ListServices()
			if err != nil {
				s.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to list %s services: %v", serviceType, err))
				return
			}
			allServices = services
		} else {
			s.sendError(w, http.StatusBadRequest, fmt.Sprintf("Unsupported service type: %s", serviceType))
			return
		}
	} else {
		// List all services from all managers
		for _, manager := range s.managers {
			services, err := manager.ListServices()
			if err != nil {
				s.logger.Warnf("Failed to list services from manager: %v", err)
				continue
			}
			allServices = append(allServices, services...)
		}
	}

	response := types.ServiceListResponse{
		Success:  true,
		Message:  "Services listed successfully",
		Services: allServices,
	}

	s.sendJSON(w, http.StatusOK, response)
}

func (s *HTTPServer) handleGetStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serviceName := vars["name"]
	serviceType := r.URL.Query().Get("type")

	manager, err := s.getServiceManager(serviceName, serviceType)
	if err != nil {
		s.sendError(w, http.StatusBadRequest, err.Error())
		return
	}

	info, err := manager.GetStatus(serviceName)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get service status: %v", err))
		return
	}

	response := types.ServiceResponse{
		Success: true,
		Message: "Service status retrieved successfully",
		Service: info,
	}

	s.sendJSON(w, http.StatusOK, response)
}

func (s *HTTPServer) handleStartService(w http.ResponseWriter, r *http.Request) {
	s.handleServiceOperation(w, r, "start")
}

func (s *HTTPServer) handleStopService(w http.ResponseWriter, r *http.Request) {
	s.handleServiceOperation(w, r, "stop")
}

func (s *HTTPServer) handleRestartService(w http.ResponseWriter, r *http.Request) {
	s.handleServiceOperation(w, r, "restart")
}

func (s *HTTPServer) handleEnableService(w http.ResponseWriter, r *http.Request) {
	s.handleServiceOperation(w, r, "enable")
}

func (s *HTTPServer) handleDisableService(w http.ResponseWriter, r *http.Request) {
	s.handleServiceOperation(w, r, "disable")
}

func (s *HTTPServer) handleServiceOperation(w http.ResponseWriter, r *http.Request, operation string) {
	vars := mux.Vars(r)
	serviceName := vars["name"]
	serviceType := r.URL.Query().Get("type")

	manager, err := s.getServiceManager(serviceName, serviceType)
	if err != nil {
		s.sendError(w, http.StatusBadRequest, err.Error())
		return
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
	default:
		s.sendError(w, http.StatusBadRequest, fmt.Sprintf("Unsupported operation: %s", operation))
		return
	}

	if operationErr != nil {
		s.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to %s service: %v", operation, operationErr))
		return
	}

	// Get updated status
	info, _ := manager.GetStatus(serviceName)

	response := types.ServiceResponse{
		Success: true,
		Message: fmt.Sprintf("Service %s %sed successfully", serviceName, operation),
		Service: info,
	}

	s.sendJSON(w, http.StatusOK, response)
}

func (s *HTTPServer) handleServiceAction(w http.ResponseWriter, r *http.Request) {
	var req types.ServiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	manager, err := s.getServiceManager(req.Name, string(req.Type))
	if err != nil {
		s.sendError(w, http.StatusBadRequest, err.Error())
		return
	}

	var operationErr error
	switch strings.ToLower(req.Action) {
	case "start":
		operationErr = manager.Start(req.Name)
	case "stop":
		operationErr = manager.Stop(req.Name)
	case "restart":
		operationErr = manager.Restart(req.Name)
	case "enable":
		operationErr = manager.Enable(req.Name)
	case "disable":
		operationErr = manager.Disable(req.Name)
	default:
		s.sendError(w, http.StatusBadRequest, fmt.Sprintf("Unsupported action: %s", req.Action))
		return
	}

	if operationErr != nil {
		s.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to %s service: %v", req.Action, operationErr))
		return
	}

	info, _ := manager.GetStatus(req.Name)

	response := types.ServiceResponse{
		Success: true,
		Message: fmt.Sprintf("Service %s %sed successfully", req.Name, req.Action),
		Service: info,
	}

	s.sendJSON(w, http.StatusOK, response)
}

func (s *HTTPServer) handleDockerLogs(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	containerName := vars["name"]

	dockerManager, exists := s.managers[types.ServiceTypeDocker].(*managers.DockerManager)
	if !exists {
		s.sendError(w, http.StatusServiceUnavailable, "Docker manager not available")
		return
	}

	lines := 100 // default
	if linesParam := r.URL.Query().Get("lines"); linesParam != "" {
		if parsedLines, err := strconv.Atoi(linesParam); err == nil {
			lines = parsedLines
		}
	}

	logs, err := dockerManager.GetLogs(containerName, lines)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get logs: %v", err))
		return
	}

	response := map[string]interface{}{
		"success": true,
		"message": "Logs retrieved successfully",
		"logs":    logs,
	}

	s.sendJSON(w, http.StatusOK, response)
}

func (s *HTTPServer) handleDockerStats(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	containerName := vars["name"]

	dockerManager, exists := s.managers[types.ServiceTypeDocker].(*managers.DockerManager)
	if !exists {
		s.sendError(w, http.StatusServiceUnavailable, "Docker manager not available")
		return
	}

	stats, err := dockerManager.GetStats(containerName)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get stats: %v", err))
		return
	}

	response := map[string]interface{}{
		"success": true,
		"message": "Stats retrieved successfully",
		"stats":   stats,
	}

	s.sendJSON(w, http.StatusOK, response)
}

func (s *HTTPServer) handleDockerRemove(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	containerName := vars["name"]
	force := r.URL.Query().Get("force") == "true"

	dockerManager, exists := s.managers[types.ServiceTypeDocker].(*managers.DockerManager)
	if !exists {
		s.sendError(w, http.StatusServiceUnavailable, "Docker manager not available")
		return
	}

	err := dockerManager.RemoveContainer(containerName, force)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to remove container: %v", err))
		return
	}

	response := map[string]interface{}{
		"success": true,
		"message": "Container removed successfully",
	}

	s.sendJSON(w, http.StatusOK, response)
}

func (s *HTTPServer) handleDockerCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ImageName     string   `json:"image_name"`
		ContainerName string   `json:"container_name"`
		Options       []string `json:"options"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	dockerManager, exists := s.managers[types.ServiceTypeDocker].(*managers.DockerManager)
	if !exists {
		s.sendError(w, http.StatusServiceUnavailable, "Docker manager not available")
		return
	}

	err := dockerManager.CreateContainer(req.ImageName, req.ContainerName, req.Options)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create container: %v", err))
		return
	}

	response := map[string]interface{}{
		"success": true,
		"message": "Container created successfully",
	}

	s.sendJSON(w, http.StatusOK, response)
}

func (s *HTTPServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status":    "healthy",
		"timestamp": utils.GetCurrentTimestamp(),
		"managers":  s.getAvailableManagers(),
	}

	s.sendJSON(w, http.StatusOK, response)
}

func (s *HTTPServer) handleInfo(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"name":     "MCP Service Manager",
		"version":  "1.0.0",
		"managers": s.getAvailableManagers(),
		"config":   s.config,
	}

	s.sendJSON(w, http.StatusOK, response)
}

func (s *HTTPServer) getServiceManager(serviceName, serviceType string) (types.ServiceManager, error) {
	if serviceType != "" {
		// Use specified service type
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

func (s *HTTPServer) getAvailableManagers() []string {
	var managerList []string
	for serviceType := range s.managers {
		managerList = append(managerList, string(serviceType))
	}
	return managerList
}

func (s *HTTPServer) sendJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

func (s *HTTPServer) sendError(w http.ResponseWriter, statusCode int, message string) {
	s.logger.Error(message)
	response := map[string]interface{}{
		"success": false,
		"message": message,
	}
	s.sendJSON(w, statusCode, response)
}

func (s *HTTPServer) Start() error {
	router := s.SetupRoutes()
	address := fmt.Sprintf("%s:%d", s.config.Server.Host, s.config.Server.Port)

	s.logger.Infof("Starting HTTP Server on %s", address)
	return http.ListenAndServe(address, router)
}