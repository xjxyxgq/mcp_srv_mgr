package managers

import (
	"fmt"
	"time"

	"nucc.com/mcp_srv_mgr/pkg/types"
)

// MockManager 用于测试的模拟管理器
type MockManager struct {
	serviceType types.ServiceType
	services    map[string]types.ServiceInfo
}

func NewMockManager(serviceType types.ServiceType) *MockManager {
	manager := &MockManager{
		serviceType: serviceType,
		services:    make(map[string]types.ServiceInfo),
	}

	// 添加一些模拟服务
	mockServices := []types.ServiceInfo{
		{
			Name:        "test-service-1",
			Type:        serviceType,
			Status:      types.StatusActive,
			Description: "Mock test service 1",
			PID:         12345,
			Uptime:      time.Hour * 2,
			LastChanged: time.Now().Add(-time.Hour * 2),
		},
		{
			Name:        "test-service-2", 
			Type:        serviceType,
			Status:      types.StatusInactive,
			Description: "Mock test service 2",
			LastChanged: time.Now().Add(-time.Minute * 30),
		},
		{
			Name:        "example-service",
			Type:        serviceType,
			Status:      types.StatusActive,
			Description: "Example mock service",
			PID:         67890,
			Uptime:      time.Hour * 5,
			LastChanged: time.Now().Add(-time.Hour * 5),
		},
	}

	for _, service := range mockServices {
		manager.services[service.Name] = service
	}

	return manager
}

func (m *MockManager) Start(serviceName string) error {
	if service, exists := m.services[serviceName]; exists {
		service.Status = types.StatusActive
		service.LastChanged = time.Now()
		service.PID = 99999 // Mock PID
		m.services[serviceName] = service
		return nil
	}
	return fmt.Errorf("service %s not found", serviceName)
}

func (m *MockManager) Stop(serviceName string) error {
	if service, exists := m.services[serviceName]; exists {
		service.Status = types.StatusInactive
		service.LastChanged = time.Now()
		service.PID = 0
		service.Uptime = 0
		m.services[serviceName] = service
		return nil
	}
	return fmt.Errorf("service %s not found", serviceName)
}

func (m *MockManager) Restart(serviceName string) error {
	if err := m.Stop(serviceName); err != nil {
		return err
	}
	time.Sleep(100 * time.Millisecond) // 模拟重启延迟
	return m.Start(serviceName)
}

func (m *MockManager) Enable(serviceName string) error {
	if _, exists := m.services[serviceName]; exists {
		// Mock 启用操作
		return nil
	}
	return fmt.Errorf("service %s not found", serviceName)
}

func (m *MockManager) Disable(serviceName string) error {
	if _, exists := m.services[serviceName]; exists {
		// Mock 禁用操作
		return nil
	}
	return fmt.Errorf("service %s not found", serviceName)
}

func (m *MockManager) GetStatus(serviceName string) (types.ServiceInfo, error) {
	if service, exists := m.services[serviceName]; exists {
		// 更新运行时间
		if service.Status == types.StatusActive && service.PID > 0 {
			service.Uptime = time.Since(service.LastChanged)
		}
		return service, nil
	}
	return types.ServiceInfo{}, fmt.Errorf("service %s not found", serviceName)
}

func (m *MockManager) ListServices() ([]types.ServiceInfo, error) {
	var services []types.ServiceInfo
	for _, service := range m.services {
		// 更新运行时间
		if service.Status == types.StatusActive && service.PID > 0 {
			service.Uptime = time.Since(service.LastChanged)
		}
		services = append(services, service)
	}
	return services, nil
}