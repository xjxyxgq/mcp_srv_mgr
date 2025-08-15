package types

import "time"

type ServiceType string

const (
	ServiceTypeSystemd ServiceType = "systemd"
	ServiceTypeSysV    ServiceType = "sysv"
	ServiceTypeDocker  ServiceType = "docker"
)

type ServiceStatus string

const (
	StatusActive   ServiceStatus = "active"
	StatusInactive ServiceStatus = "inactive"
	StatusFailed   ServiceStatus = "failed"
	StatusUnknown  ServiceStatus = "unknown"
)

type ServiceInfo struct {
	Name        string        `json:"name"`
	Type        ServiceType   `json:"type"`
	Status      ServiceStatus `json:"status"`
	Description string        `json:"description,omitempty"`
	PID         int           `json:"pid,omitempty"`
	Uptime      time.Duration `json:"uptime,omitempty"`
	LastChanged time.Time     `json:"last_changed,omitempty"`
}

type ServiceRequest struct {
	Name   string      `json:"name"`
	Type   ServiceType `json:"type,omitempty"`
	Action string      `json:"action"`
}

type ServiceResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Service ServiceInfo `json:"service,omitempty"`
}

type ServiceListResponse struct {
	Success  bool          `json:"success"`
	Message  string        `json:"message"`
	Services []ServiceInfo `json:"services"`
}

type ServiceManager interface {
	Start(serviceName string) error
	Stop(serviceName string) error
	Restart(serviceName string) error
	GetStatus(serviceName string) (ServiceInfo, error)
	ListServices() ([]ServiceInfo, error)
	Enable(serviceName string) error
	Disable(serviceName string) error
}