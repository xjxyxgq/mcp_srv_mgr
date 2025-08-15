package managers

import (
	"testing"

	"nucc.com/mcp_srv_mgr/pkg/types"
)

func TestNewSystemdManager(t *testing.T) {
	manager := NewSystemdManager()
	if manager == nil {
		t.Fatal("Expected SystemdManager instance, got nil")
	}
}

func TestSystemdManager_Interface(t *testing.T) {
	var _ types.ServiceManager = (*SystemdManager)(nil)
}

// Mock测试 - 测试systemd不可用时的行为
func TestIsSystemdAvailable(t *testing.T) {
	// 注意：这个测试依赖于系统环境
	// 在CI环境中可能需要mock
	available := IsSystemdAvailable()
	t.Logf("systemd available: %t", available)
	
	// 这里我们只是记录结果，不强制断言
	// 因为测试环境可能没有systemd
}

// 测试服务信息解析
func TestSystemdManager_ParseServiceStatus(t *testing.T) {
	manager := NewSystemdManager()
	
	// 测试一个可能存在的系统服务
	// 使用一个相对通用的服务名
	serviceName := "systemd-timesyncd" // 这是一个比较常见的systemd服务
	
	// 如果systemd不可用，跳过测试
	if !IsSystemdAvailable() {
		t.Skip("systemd not available, skipping systemd manager tests")
	}
	
	info, err := manager.GetStatus(serviceName)
	
	// 验证返回的信息结构
	if info.Name != serviceName {
		t.Errorf("Expected service name %s, got %s", serviceName, info.Name)
	}
	
	if info.Type != types.ServiceTypeSystemd {
		t.Errorf("Expected service type %s, got %s", types.ServiceTypeSystemd, info.Type)
	}
	
	// 服务状态应该是有效的状态值之一
	validStatuses := []types.ServiceStatus{
		types.StatusActive, types.StatusInactive, 
		types.StatusFailed, types.StatusUnknown,
	}
	
	found := false
	for _, status := range validStatuses {
		if info.Status == status {
			found = true
			break
		}
	}
	
	if !found {
		t.Errorf("Invalid service status: %s", info.Status)
	}
	
	// 如果有错误，记录但不失败测试（因为服务可能不存在）
	if err != nil {
		t.Logf("GetStatus returned error (service may not exist): %v", err)
	}
}

func TestSystemdManager_ListServices(t *testing.T) {
	// 如果systemd不可用，跳过测试
	if !IsSystemdAvailable() {
		t.Skip("systemd not available, skipping systemd manager tests")
	}
	
	manager := NewSystemdManager()
	services, err := manager.ListServices()
	
	if err != nil {
		t.Fatalf("Failed to list systemd services: %v", err)
	}
	
	// 验证返回的服务列表
	t.Logf("Found %d systemd services", len(services))
	
	// 验证每个服务的基本字段
	for i, service := range services {
		if service.Name == "" {
			t.Errorf("Service %d has empty name", i)
		}
		if service.Type != types.ServiceTypeSystemd {
			t.Errorf("Service %d has wrong type: expected %s, got %s", 
				i, types.ServiceTypeSystemd, service.Type)
		}
		
		// 限制检查前10个服务，避免测试运行时间过长
		if i >= 10 {
			break
		}
	}
}

// 测试基本操作（这些测试需要root权限，通常在CI中跳过）
func TestSystemdManager_BasicOperations(t *testing.T) {
	if !IsSystemdAvailable() {
		t.Skip("systemd not available, skipping systemd manager tests")
	}
	
	manager := NewSystemdManager()
	
	// 选择一个安全的测试服务（通常不会影响系统）
	testService := "systemd-timesyncd"
	
	// 测试获取状态
	_, err := manager.GetStatus(testService)
	if err != nil {
		t.Logf("GetStatus failed (expected for non-root): %v", err)
	}
	
	// 注意：Start, Stop, Restart, Enable, Disable 操作通常需要root权限
	// 在单元测试中，我们只测试这些方法不会panic
	
	// 测试启动（预期会失败，因为没有root权限）
	err = manager.Start(testService)
	if err != nil {
		t.Logf("Start failed (expected for non-root): %v", err)
	}
	
	// 测试停止（预期会失败，因为没有root权限）
	err = manager.Stop(testService)
	if err != nil {
		t.Logf("Stop failed (expected for non-root): %v", err)
	}
	
	// 测试重启（预期会失败，因为没有root权限）
	err = manager.Restart(testService)
	if err != nil {
		t.Logf("Restart failed (expected for non-root): %v", err)
	}
	
	// 测试启用（预期会失败，因为没有root权限）
	err = manager.Enable(testService)
	if err != nil {
		t.Logf("Enable failed (expected for non-root): %v", err)
	}
	
	// 测试禁用（预期会失败，因为没有root权限）
	err = manager.Disable(testService)
	if err != nil {
		t.Logf("Disable failed (expected for non-root): %v", err)
	}
}

// 测试不存在的服务
func TestSystemdManager_NonexistentService(t *testing.T) {
	if !IsSystemdAvailable() {
		t.Skip("systemd not available, skipping systemd manager tests")
	}
	
	manager := NewSystemdManager()
	nonexistentService := "definitely-does-not-exist-service-12345"
	
	// 获取不存在服务的状态应该返回错误或失败状态
	info, err := manager.GetStatus(nonexistentService)
	
	if err == nil && info.Status != types.StatusFailed && info.Status != types.StatusUnknown {
		t.Errorf("Expected error or failed/unknown status for nonexistent service, got status: %s", info.Status)
	}
}

// Benchmark测试
func BenchmarkSystemdManager_GetStatus(b *testing.B) {
	if !IsSystemdAvailable() {
		b.Skip("systemd not available, skipping benchmark")
	}
	
	manager := NewSystemdManager()
	serviceName := "systemd-timesyncd"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = manager.GetStatus(serviceName)
	}
}

func BenchmarkSystemdManager_ListServices(b *testing.B) {
	if !IsSystemdAvailable() {
		b.Skip("systemd not available, skipping benchmark")
	}
	
	manager := NewSystemdManager()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = manager.ListServices()
	}
}