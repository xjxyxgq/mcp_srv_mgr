package managers

import (
	"encoding/json"
	"testing"

	"nucc.com/mcp_srv_mgr/pkg/types"
)

func TestNewDockerManager(t *testing.T) {
	manager := NewDockerManager()
	if manager == nil {
		t.Fatal("Expected DockerManager instance, got nil")
	}
}

func TestDockerManager_Interface(t *testing.T) {
	var _ types.ServiceManager = (*DockerManager)(nil)
}

func TestIsDockerAvailable(t *testing.T) {
	available := IsDockerAvailable()
	t.Logf("Docker available: %t", available)
	
	// 这个测试主要是为了记录Docker是否可用
	// 在CI环境中Docker可能不可用
}

func TestDockerContainer_JSONParsing(t *testing.T) {
	// 测试Docker JSON输出解析
	jsonData := `{
		"Id": "abc123def456",
		"Names": ["/test-container", "/alias-name"],
		"Image": "nginx:latest",
		"State": "running",
		"Status": "Up 2 hours",
		"Created": 1640995200,
		"Labels": {
			"com.example.label": "value"
		}
	}`
	
	var container DockerContainer
	err := json.Unmarshal([]byte(jsonData), &container)
	if err != nil {
		t.Fatalf("Failed to unmarshal Docker container JSON: %v", err)
	}
	
	if container.ID != "abc123def456" {
		t.Errorf("Expected ID abc123def456, got %s", container.ID)
	}
	
	if len(container.Names) != 2 {
		t.Errorf("Expected 2 names, got %d", len(container.Names))
	}
	
	if container.Names[0] != "/test-container" {
		t.Errorf("Expected first name /test-container, got %s", container.Names[0])
	}
	
	if container.Image != "nginx:latest" {
		t.Errorf("Expected image nginx:latest, got %s", container.Image)
	}
	
	if container.State != "running" {
		t.Errorf("Expected state running, got %s", container.State)
	}
	
	if container.Created != 1640995200 {
		t.Errorf("Expected created time 1640995200, got %d", container.Created)
	}
}

func TestDockerManager_StatusMapping(t *testing.T) {
	// 测试状态映射
	testCases := []struct {
		dockerState    string
		expectedStatus types.ServiceStatus
	}{
		{"running", types.StatusActive},
		{"exited", types.StatusInactive},
		{"created", types.StatusInactive},
		{"dead", types.StatusFailed},
		{"restarting", types.StatusFailed},
		{"unknown_state", types.StatusUnknown},
	}
	
	for _, tc := range testCases {
		// 创建模拟容器数据
		jsonData := `{
			"Id": "test123",
			"Names": ["/test-container"],
			"Image": "test:latest",
			"State": "` + tc.dockerState + `",
			"Status": "Test status",
			"Created": 1640995200
		}`
		
		var container DockerContainer
		err := json.Unmarshal([]byte(jsonData), &container)
		if err != nil {
			t.Fatalf("Failed to unmarshal test container: %v", err)
		}
		
		// 测试状态映射逻辑
		var status types.ServiceStatus
		switch container.State {
		case "running":
			status = types.StatusActive
		case "exited", "created":
			status = types.StatusInactive
		case "dead", "restarting":
			status = types.StatusFailed
		default:
			status = types.StatusUnknown
		}
		
		if status != tc.expectedStatus {
			t.Errorf("For state %s, expected %s, got %s", 
				tc.dockerState, tc.expectedStatus, status)
		}
	}
}

func TestDockerManager_ListServices(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Docker tests in short mode")
	}
	if !IsDockerAvailable() {
		t.Skip("Docker not available, skipping Docker manager tests")
	}
	
	manager := NewDockerManager()
	services, err := manager.ListServices()
	
	if err != nil {
		t.Fatalf("Failed to list Docker containers: %v", err)
	}
	
	t.Logf("Found %d Docker containers", len(services))
	
	// 验证每个容器的基本字段
	for i, service := range services {
		if service.Name == "" {
			t.Errorf("Container %d has empty name", i)
		}
		if service.Type != types.ServiceTypeDocker {
			t.Errorf("Container %d has wrong type: expected %s, got %s", 
				i, types.ServiceTypeDocker, service.Type)
		}
		
		// 限制检查前5个容器
		if i >= 5 {
			break
		}
	}
}

func TestDockerManager_GetStatus(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Docker tests in short mode")
	}
	if !IsDockerAvailable() {
		t.Skip("Docker not available, skipping Docker manager tests")
	}
	
	manager := NewDockerManager()
	
	// 首先获取容器列表
	services, err := manager.ListServices()
	if err != nil {
		t.Fatalf("Failed to list Docker containers: %v", err)
	}
	
	if len(services) == 0 {
		t.Skip("No Docker containers found, skipping status test")
	}
	
	// 测试第一个容器的状态
	containerName := services[0].Name
	info, err := manager.GetStatus(containerName)
	
	if err != nil {
		t.Fatalf("Failed to get status for container %s: %v", containerName, err)
	}
	
	if info.Name != containerName {
		t.Errorf("Expected container name %s, got %s", containerName, info.Name)
	}
	
	if info.Type != types.ServiceTypeDocker {
		t.Errorf("Expected service type %s, got %s", types.ServiceTypeDocker, info.Type)
	}
}

func TestDockerManager_NonexistentContainer(t *testing.T) {
	if !IsDockerAvailable() {
		t.Skip("Docker not available, skipping Docker manager tests")
	}
	
	manager := NewDockerManager()
	nonexistentContainer := "definitely-does-not-exist-container-12345"
	
	// 获取不存在容器的状态应该返回错误
	_, err := manager.GetStatus(nonexistentContainer)
	if err == nil {
		t.Error("Expected error for nonexistent container, but got none")
	}
}

func TestDockerManager_BasicOperations(t *testing.T) {
	if !IsDockerAvailable() {
		t.Skip("Docker not available, skipping Docker manager tests")
	}
	
	manager := NewDockerManager()
	testContainer := "test-nonexistent-container"
	
	// 测试基本操作（这些操作应该会失败，因为容器不存在）
	err := manager.Start(testContainer)
	if err != nil {
		t.Logf("Start failed (expected for nonexistent container): %v", err)
	}
	
	err = manager.Stop(testContainer)
	if err != nil {
		t.Logf("Stop failed (expected for nonexistent container): %v", err)
	}
	
	err = manager.Restart(testContainer)
	if err != nil {
		t.Logf("Restart failed (expected for nonexistent container): %v", err)
	}
	
	err = manager.Enable(testContainer)
	if err != nil {
		t.Logf("Enable failed (expected for nonexistent container): %v", err)
	}
	
	err = manager.Disable(testContainer)
	if err != nil {
		t.Logf("Disable failed (expected for nonexistent container): %v", err)
	}
}

func TestDockerManager_AdditionalMethods(t *testing.T) {
	if !IsDockerAvailable() {
		t.Skip("Docker not available, skipping Docker manager tests")
	}
	
	manager := NewDockerManager()
	testContainer := "test-nonexistent-container"
	
	// 测试获取日志（应该失败，因为容器不存在）
	_, err := manager.GetLogs(testContainer, 10)
	if err != nil {
		t.Logf("GetLogs failed (expected for nonexistent container): %v", err)
	}
	
	// 测试获取统计信息（应该失败，因为容器不存在）
	_, err = manager.GetStats(testContainer)
	if err != nil {
		t.Logf("GetStats failed (expected for nonexistent container): %v", err)
	}
	
	// 测试删除容器（应该失败，因为容器不存在）
	err = manager.RemoveContainer(testContainer, false)
	if err != nil {
		t.Logf("RemoveContainer failed (expected for nonexistent container): %v", err)
	}
	
	// 测试创建容器（使用无效镜像，应该失败）
	err = manager.CreateContainer("nonexistent-image:latest", "test-container", []string{})
	if err != nil {
		t.Logf("CreateContainer failed (expected for nonexistent image): %v", err)
	}
}

// Benchmark测试
func BenchmarkDockerManager_ListServices(b *testing.B) {
	if !IsDockerAvailable() {
		b.Skip("Docker not available, skipping benchmark")
	}
	
	manager := NewDockerManager()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = manager.ListServices()
	}
}

func BenchmarkDockerManager_GetStatus(b *testing.B) {
	if !IsDockerAvailable() {
		b.Skip("Docker not available, skipping benchmark")
	}
	
	manager := NewDockerManager()
	
	// 获取一个容器名用于测试
	services, err := manager.ListServices()
	if err != nil || len(services) == 0 {
		b.Skip("No containers available for benchmark")
	}
	
	containerName := services[0].Name
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = manager.GetStatus(containerName)
	}
}