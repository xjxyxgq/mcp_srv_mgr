package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_Default(t *testing.T) {
	// 测试加载默认配置
	config, err := Load("")
	if err != nil {
		t.Fatalf("Failed to load default config: %v", err)
	}

	// 验证默认值
	if config.Server.Host != "127.0.0.1" {
		t.Errorf("Expected default host 127.0.0.1, got %s", config.Server.Host)
	}
	if config.Server.Port != 8080 {
		t.Errorf("Expected default port 8080, got %d", config.Server.Port)
	}
	if config.Log.Level != "info" {
		t.Errorf("Expected default log level info, got %s", config.Log.Level)
	}
	if config.Log.Format != "json" {
		t.Errorf("Expected default log format json, got %s", config.Log.Format)
	}
	if config.Log.Output != "stdout" {
		t.Errorf("Expected default log output stdout, got %s", config.Log.Output)
	}
}

func TestLoad_FromFile(t *testing.T) {
	// 创建临时配置文件
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "test_config.yaml")

	configContent := `
server:
  host: "0.0.0.0"
  port: 9090

log:
  level: "debug"
  format: "text"
  output: "stderr"
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	// 加载配置文件
	config, err := Load(configFile)
	if err != nil {
		t.Fatalf("Failed to load config from file: %v", err)
	}

	// 验证配置值
	if config.Server.Host != "0.0.0.0" {
		t.Errorf("Expected host 0.0.0.0, got %s", config.Server.Host)
	}
	if config.Server.Port != 9090 {
		t.Errorf("Expected port 9090, got %d", config.Server.Port)
	}
	if config.Log.Level != "debug" {
		t.Errorf("Expected log level debug, got %s", config.Log.Level)
	}
	if config.Log.Format != "text" {
		t.Errorf("Expected log format text, got %s", config.Log.Format)
	}
	if config.Log.Output != "stderr" {
		t.Errorf("Expected log output stderr, got %s", config.Log.Output)
	}
}

func TestLoad_EnvironmentOverrides(t *testing.T) {
	// 设置环境变量
	originalEnvs := map[string]string{
		"MCP_HOST":       os.Getenv("MCP_HOST"),
		"MCP_PORT":       os.Getenv("MCP_PORT"),
		"MCP_LOG_LEVEL":  os.Getenv("MCP_LOG_LEVEL"),
		"MCP_LOG_FORMAT": os.Getenv("MCP_LOG_FORMAT"),
	}

	// 设置测试环境变量
	os.Setenv("MCP_HOST", "192.168.1.100")
	os.Setenv("MCP_PORT", "7777")
	os.Setenv("MCP_LOG_LEVEL", "warn")
	os.Setenv("MCP_LOG_FORMAT", "text")

	// 清理函数
	defer func() {
		for key, value := range originalEnvs {
			if value == "" {
				os.Unsetenv(key)
			} else {
				os.Setenv(key, value)
			}
		}
	}()

	// 加载配置
	config, err := Load("")
	if err != nil {
		t.Fatalf("Failed to load config with env overrides: %v", err)
	}

	// 验证环境变量覆盖
	if config.Server.Host != "192.168.1.100" {
		t.Errorf("Expected host from env 192.168.1.100, got %s", config.Server.Host)
	}
	if config.Server.Port != 7777 {
		t.Errorf("Expected port from env 7777, got %d", config.Server.Port)
	}
	if config.Log.Level != "warn" {
		t.Errorf("Expected log level from env warn, got %s", config.Log.Level)
	}
	if config.Log.Format != "text" {
		t.Errorf("Expected log format from env text, got %s", config.Log.Format)
	}
}

func TestLoad_InvalidFile(t *testing.T) {
	// 测试不存在的文件
	_, err := Load("/nonexistent/config.yaml")
	if err != nil {
		t.Logf("Expected error for nonexistent file: %v", err)
	}

	// 测试无效的YAML文件
	tempDir := t.TempDir()
	invalidFile := filepath.Join(tempDir, "invalid.yaml")

	invalidContent := `
server:
  host: "test"
  port: invalid_port
log:
  - invalid: yaml: structure
`

	err = os.WriteFile(invalidFile, []byte(invalidContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write invalid config file: %v", err)
	}

	_, err = Load(invalidFile)
	if err == nil {
		t.Error("Expected error for invalid YAML file, but got none")
	}
}

func TestConfig_SaveToFile(t *testing.T) {
	config := &Config{
		Server: ServerConfig{
			Host: "test.example.com",
			Port: 8888,
		},
		Log: LogConfig{
			Level:  "error",
			Format: "json",
			Output: "stdout",
		},
	}

	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "saved_config.yaml")

	// 保存配置到文件
	err := config.SaveToFile(configFile)
	if err != nil {
		t.Fatalf("Failed to save config to file: %v", err)
	}

	// 验证文件是否存在
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		t.Error("Config file was not created")
	}

	// 重新加载配置并验证
	loadedConfig, err := Load(configFile)
	if err != nil {
		t.Fatalf("Failed to load saved config: %v", err)
	}

	if loadedConfig.Server.Host != config.Server.Host {
		t.Errorf("Host mismatch: expected %s, got %s", config.Server.Host, loadedConfig.Server.Host)
	}
	if loadedConfig.Server.Port != config.Server.Port {
		t.Errorf("Port mismatch: expected %d, got %d", config.Server.Port, loadedConfig.Server.Port)
	}
	if loadedConfig.Log.Level != config.Log.Level {
		t.Errorf("Log level mismatch: expected %s, got %s", config.Log.Level, loadedConfig.Log.Level)
	}
}

func TestConfig_SaveToFile_InvalidPath(t *testing.T) {
	config := &Config{
		Server: ServerConfig{Host: "test", Port: 8080},
		Log:    LogConfig{Level: "info", Format: "json", Output: "stdout"},
	}

	// 测试无效路径
	err := config.SaveToFile("/invalid/path/that/does/not/exist/config.yaml")
	if err == nil {
		t.Error("Expected error for invalid file path, but got none")
	}
}

func TestConfig_PartialYAML(t *testing.T) {
	// 测试部分配置文件（只有server部分）
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "partial_config.yaml")

	partialContent := `
server:
  host: "partial.test.com"
  port: 5555
`

	err := os.WriteFile(configFile, []byte(partialContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write partial config file: %v", err)
	}

	config, err := Load(configFile)
	if err != nil {
		t.Fatalf("Failed to load partial config: %v", err)
	}

	// 验证服务器配置被加载
	if config.Server.Host != "partial.test.com" {
		t.Errorf("Expected host partial.test.com, got %s", config.Server.Host)
	}
	if config.Server.Port != 5555 {
		t.Errorf("Expected port 5555, got %d", config.Server.Port)
	}

	// 验证日志配置使用默认值
	if config.Log.Level != "info" {
		t.Errorf("Expected default log level info, got %s", config.Log.Level)
	}
	if config.Log.Format != "json" {
		t.Errorf("Expected default log format json, got %s", config.Log.Format)
	}
}

func TestConfig_EmptyFile(t *testing.T) {
	// 测试空配置文件
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "empty_config.yaml")

	err := os.WriteFile(configFile, []byte(""), 0644)
	if err != nil {
		t.Fatalf("Failed to write empty config file: %v", err)
	}

	config, err := Load(configFile)
	if err != nil {
		t.Fatalf("Failed to load empty config: %v", err)
	}

	// 验证所有值都是默认值
	if config.Server.Host != "127.0.0.1" {
		t.Errorf("Expected default host, got %s", config.Server.Host)
	}
	if config.Server.Port != 8080 {
		t.Errorf("Expected default port, got %d", config.Server.Port)
	}
}