package config

import (
	"fmt"
	"os"
	"strconv"

	"gopkg.in/yaml.v2"
)

type Config struct {
	Server ServerConfig `yaml:"server"`
	Log    LogConfig    `yaml:"log"`
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type LogConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
	Output string `yaml:"output"`
}

func Load(configPath string) (*Config, error) {
	// Default configuration
	config := &Config{
		Server: ServerConfig{
			Host: "127.0.0.1",
			Port: 8080,
		},
		Log: LogConfig{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		},
	}

	// Load from file if exists
	if configPath != "" {
		if _, err := os.Stat(configPath); err == nil {
			data, err := os.ReadFile(configPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read config file: %v", err)
			}

			if err := yaml.Unmarshal(data, config); err != nil {
				return nil, fmt.Errorf("failed to parse config file: %v", err)
			}
		}
	}

	// Override with environment variables
	if host := os.Getenv("MCP_HOST"); host != "" {
		config.Server.Host = host
	}

	if port := os.Getenv("MCP_PORT"); port != "" {
		if portNum, err := strconv.Atoi(port); err == nil {
			config.Server.Port = portNum
		}
	}

	if logLevel := os.Getenv("MCP_LOG_LEVEL"); logLevel != "" {
		config.Log.Level = logLevel
	}

	if logFormat := os.Getenv("MCP_LOG_FORMAT"); logFormat != "" {
		config.Log.Format = logFormat
	}

	return config, nil
}

func (c *Config) SaveToFile(filename string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %v", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %v", err)
	}

	return nil
}