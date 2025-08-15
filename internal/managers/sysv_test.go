package managers

import (
	"testing"
	"time"

	"nucc.com/mcp_srv_mgr/pkg/types"
)

func TestNewSysVManager(t *testing.T) {
	manager := NewSysVManager()
	if manager == nil {
		t.Fatal("Expected SysVManager instance, got nil")
	}
	
	if manager.initDPath != "/etc/init.d" {
		t.Errorf("Expected initDPath /etc/init.d, got %s", manager.initDPath)
	}
}

func TestSysVManager_Interface(t *testing.T) {
	var _ types.ServiceManager = (*SysVManager)(nil)
}

func TestIsSysVAvailable(t *testing.T) {
	available := IsSysVAvailable()
	t.Logf("SysV available: %t", available)
	
	// 这个测试主要是为了记录SysV是否可用
	// 在现代Linux发行版中，SysV可能不可用
}

func TestSysVManager_ServiceExists(t *testing.T) {
	manager := NewSysVManager()
	
	// 测试不存在的服务
	exists := manager.serviceExists("definitely-does-not-exist-service-12345")
	if exists {
		t.Error("Expected false for nonexistent service, got true")
	}
	
	// 如果SysV可用，测试一些常见的服务
	if IsSysVAvailable() {
		// 这些是一些可能存在的传统服务
		possibleServices := []string{"networking", "cron", "ssh", "rsyslog"}
		
		for _, service := range possibleServices {
			exists := manager.serviceExists(service)
			t.Logf("Service %s exists: %t", service, exists)
		}
	}
}

func TestSysVManager_IsNonServiceFile(t *testing.T) {
	manager := NewSysVManager()
	
	// 测试非服务文件
	nonServiceFiles := []string{
		"README", "skeleton", "rcS", "rc", "functions",
		"halt", "killall", "single", "reboot",
	}
	
	for _, filename := range nonServiceFiles {
		if !manager.isNonServiceFile(filename) {
			t.Errorf("Expected %s to be recognized as non-service file", filename)
		}
	}
	
	// 测试可能的服务文件
	serviceFiles := []string{
		"nginx", "apache2", "mysql", "postgresql", "ssh",
	}
	
	for _, filename := range serviceFiles {
		if manager.isNonServiceFile(filename) {
			t.Errorf("Expected %s to be recognized as service file", filename)
		}
	}
}

func TestSysVManager_ExtractPIDFromStatus(t *testing.T) {
	manager := NewSysVManager()
	
	testCases := []struct {
		output      string
		expectedPID int
	}{
		{"nginx is running with PID 1234", 1234},
		{"service (pid 5678) is running", 5678},
		{"Status: PID: 9999", 9999},
		{"PID 4321 - service is active", 4321},
		{"no pid information", 0},
		{"", 0},
		{"service is stopped", 0},
	}
	
	for _, tc := range testCases {
		pid := manager.extractPIDFromStatus(tc.output)
		if pid != tc.expectedPID {
			t.Errorf("For output %q, expected PID %d, got %d", 
				tc.output, tc.expectedPID, pid)
		}
	}
}

func TestSysVManager_ParseEtime(t *testing.T) {
	manager := NewSysVManager()
	
	testCases := []struct {
		etime    string
		expected time.Duration
	}{
		{"01:30", 1*time.Minute + 30*time.Second},
		{"02:15:45", 2*time.Hour + 15*time.Minute + 45*time.Second},
		{"1-12:30:45", 24*time.Hour + 12*time.Hour + 30*time.Minute + 45*time.Second},
		{"00:00", 0},
		{"", 0},
	}
	
	for _, tc := range testCases {
		duration := manager.parseEtime(tc.etime)
		if duration != tc.expected {
			t.Errorf("For etime %q, expected %v, got %v", 
				tc.etime, tc.expected, duration)
		}
	}
}

func TestSysVManager_HasCommands(t *testing.T) {
	manager := NewSysVManager()
	
	// 测试命令检查
	hasChkconfig := manager.hasChkconfig()
	hasUpdateRcd := manager.hasUpdateRcd()
	
	t.Logf("chkconfig available: %t", hasChkconfig)
	t.Logf("update-rc.d available: %t", hasUpdateRcd)
	
	// 至少应该有一个命令可用（在大多数Linux系统上）
	if !hasChkconfig && !hasUpdateRcd {
		t.Log("Neither chkconfig nor update-rc.d available (may be expected on some systems)")
	}
}

func TestSysVManager_ListServices(t *testing.T) {
	if !IsSysVAvailable() {
		t.Skip("SysV not available, skipping SysV manager tests")
	}
	
	manager := NewSysVManager()
	services, err := manager.ListServices()
	
	if err != nil {
		t.Fatalf("Failed to list SysV services: %v", err)
	}
	
	t.Logf("Found %d SysV services", len(services))
	
	// 验证每个服务的基本字段
	for i, service := range services {
		if service.Name == "" {
			t.Errorf("Service %d has empty name", i)
		}
		if service.Type != types.ServiceTypeSysV {
			t.Errorf("Service %d has wrong type: expected %s, got %s", 
				i, types.ServiceTypeSysV, service.Type)
		}
		
		// 限制检查前10个服务
		if i >= 10 {
			break
		}
	}
}

func TestSysVManager_GetStatus(t *testing.T) {
	if !IsSysVAvailable() {
		t.Skip("SysV not available, skipping SysV manager tests")
	}
	
	manager := NewSysVManager()
	
	// 首先获取服务列表
	services, err := manager.ListServices()
	if err != nil {
		t.Fatalf("Failed to list SysV services: %v", err)
	}
	
	if len(services) == 0 {
		t.Skip("No SysV services found, skipping status test")
	}
	
	// 测试第一个服务的状态
	serviceName := services[0].Name
	info, err := manager.GetStatus(serviceName)
	
	// 即使出错也要检查基本字段
	if info.Name != serviceName {
		t.Errorf("Expected service name %s, got %s", serviceName, info.Name)
	}
	
	if info.Type != types.ServiceTypeSysV {
		t.Errorf("Expected service type %s, got %s", types.ServiceTypeSysV, info.Type)
	}
	
	if err != nil {
		t.Logf("GetStatus returned error (may be expected): %v", err)
	}
}

func TestSysVManager_BasicOperations(t *testing.T) {
	if !IsSysVAvailable() {
		t.Skip("SysV not available, skipping SysV manager tests")
	}
	
	manager := NewSysVManager()
	testService := "nonexistent-service-12345"
	
	// 测试基本操作（这些操作应该会失败，因为服务不存在或需要权限）
	err := manager.Start(testService)
	if err == nil {
		t.Error("Expected error for nonexistent service start, but got none")
	}
	
	err = manager.Stop(testService)
	if err == nil {
		t.Error("Expected error for nonexistent service stop, but got none")
	}
	
	err = manager.Restart(testService)
	if err == nil {
		t.Error("Expected error for nonexistent service restart, but got none")
	}
	
	err = manager.Enable(testService)
	if err == nil {
		t.Error("Expected error for nonexistent service enable, but got none")
	}
	
	err = manager.Disable(testService)
	if err == nil {
		t.Error("Expected error for nonexistent service disable, but got none")
	}
}

func TestSysVManager_NonexistentService(t *testing.T) {
	if !IsSysVAvailable() {
		t.Skip("SysV not available, skipping SysV manager tests")
	}
	
	manager := NewSysVManager()
	nonexistentService := "definitely-does-not-exist-service-12345"
	
	// 获取不存在服务的状态应该返回错误
	_, err := manager.GetStatus(nonexistentService)
	if err == nil {
		t.Error("Expected error for nonexistent service, but got none")
	}
}

// Benchmark测试
func BenchmarkSysVManager_ListServices(b *testing.B) {
	if !IsSysVAvailable() {
		b.Skip("SysV not available, skipping benchmark")
	}
	
	manager := NewSysVManager()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = manager.ListServices()
	}
}

func BenchmarkSysVManager_GetStatus(b *testing.B) {
	if !IsSysVAvailable() {
		b.Skip("SysV not available, skipping benchmark")
	}
	
	manager := NewSysVManager()
	
	// 获取一个服务名用于测试
	services, err := manager.ListServices()
	if err != nil || len(services) == 0 {
		b.Skip("No services available for benchmark")
	}
	
	serviceName := services[0].Name
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = manager.GetStatus(serviceName)
	}
}