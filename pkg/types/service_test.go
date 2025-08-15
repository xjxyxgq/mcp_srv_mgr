package types

import (
	"encoding/json"
	"testing"
	"time"
)

func TestServiceInfo_JSON(t *testing.T) {
	service := ServiceInfo{
		Name:        "nginx",
		Type:        ServiceTypeSystemd,
		Status:      StatusActive,
		Description: "Web server",
		PID:         1234,
		Uptime:      2 * time.Hour,
		LastChanged: time.Date(2023, 1, 1, 10, 0, 0, 0, time.UTC),
	}

	// 测试JSON序列化
	data, err := json.Marshal(service)
	if err != nil {
		t.Fatalf("Failed to marshal ServiceInfo: %v", err)
	}

	// 测试JSON反序列化
	var decoded ServiceInfo
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal ServiceInfo: %v", err)
	}

	// 验证字段
	if decoded.Name != service.Name {
		t.Errorf("Name mismatch: expected %s, got %s", service.Name, decoded.Name)
	}
	if decoded.Type != service.Type {
		t.Errorf("Type mismatch: expected %s, got %s", service.Type, decoded.Type)
	}
	if decoded.Status != service.Status {
		t.Errorf("Status mismatch: expected %s, got %s", service.Status, decoded.Status)
	}
	if decoded.PID != service.PID {
		t.Errorf("PID mismatch: expected %d, got %d", service.PID, decoded.PID)
	}
}

func TestServiceRequest_Validation(t *testing.T) {
	tests := []struct {
		name     string
		request  ServiceRequest
		isValid  bool
	}{
		{
			name: "Valid systemd request",
			request: ServiceRequest{
				Name:   "nginx",
				Type:   ServiceTypeSystemd,
				Action: "start",
			},
			isValid: true,
		},
		{
			name: "Valid docker request",
			request: ServiceRequest{
				Name:   "web-container",
				Type:   ServiceTypeDocker,
				Action: "stop",
			},
			isValid: true,
		},
		{
			name: "Empty name",
			request: ServiceRequest{
				Name:   "",
				Type:   ServiceTypeSystemd,
				Action: "start",
			},
			isValid: false,
		},
		{
			name: "Empty action",
			request: ServiceRequest{
				Name:   "nginx",
				Type:   ServiceTypeSystemd,
				Action: "",
			},
			isValid: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// 测试JSON序列化/反序列化
			data, err := json.Marshal(test.request)
			if err != nil {
				t.Fatalf("Failed to marshal request: %v", err)
			}

			var decoded ServiceRequest
			err = json.Unmarshal(data, &decoded)
			if err != nil {
				t.Fatalf("Failed to unmarshal request: %v", err)
			}

			// 验证基本字段
			if decoded.Name != test.request.Name {
				t.Errorf("Name mismatch: expected %s, got %s", test.request.Name, decoded.Name)
			}

			// 简单的验证逻辑
			isValid := decoded.Name != "" && decoded.Action != ""
			if isValid != test.isValid {
				t.Errorf("Validation mismatch: expected %t, got %t", test.isValid, isValid)
			}
		})
	}
}

func TestServiceResponse_Success(t *testing.T) {
	service := ServiceInfo{
		Name:   "nginx",
		Type:   ServiceTypeSystemd,
		Status: StatusActive,
	}

	response := ServiceResponse{
		Success: true,
		Message: "Operation successful",
		Service: service,
	}

	// 测试JSON序列化
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("Failed to marshal response: %v", err)
	}

	// 验证JSON包含预期字段
	var jsonMap map[string]interface{}
	err = json.Unmarshal(data, &jsonMap)
	if err != nil {
		t.Fatalf("Failed to unmarshal to map: %v", err)
	}

	if jsonMap["success"] != true {
		t.Error("Expected success to be true")
	}

	if jsonMap["message"] != "Operation successful" {
		t.Error("Message field mismatch")
	}

	serviceMap := jsonMap["service"].(map[string]interface{})
	if serviceMap["name"] != "nginx" {
		t.Error("Service name mismatch")
	}
}

func TestServiceListResponse(t *testing.T) {
	services := []ServiceInfo{
		{Name: "nginx", Type: ServiceTypeSystemd, Status: StatusActive},
		{Name: "mysql", Type: ServiceTypeSystemd, Status: StatusInactive},
		{Name: "web-app", Type: ServiceTypeDocker, Status: StatusActive},
	}

	response := ServiceListResponse{
		Success:  true,
		Message:  "Services listed",
		Services: services,
	}

	// 测试JSON序列化
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("Failed to marshal list response: %v", err)
	}

	// 测试反序列化
	var decoded ServiceListResponse
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal list response: %v", err)
	}

	if len(decoded.Services) != len(services) {
		t.Errorf("Services count mismatch: expected %d, got %d", len(services), len(decoded.Services))
	}

	for i, service := range decoded.Services {
		if service.Name != services[i].Name {
			t.Errorf("Service %d name mismatch: expected %s, got %s", i, services[i].Name, service.Name)
		}
	}
}

func TestServiceStatus_Constants(t *testing.T) {
	// 验证状态常量
	statuses := []ServiceStatus{
		StatusActive,
		StatusInactive,
		StatusFailed,
		StatusUnknown,
	}

	expectedValues := []string{"active", "inactive", "failed", "unknown"}

	for i, status := range statuses {
		if string(status) != expectedValues[i] {
			t.Errorf("Status constant %d mismatch: expected %s, got %s", i, expectedValues[i], string(status))
		}
	}
}

func TestServiceType_Constants(t *testing.T) {
	// 验证类型常量
	types := []ServiceType{
		ServiceTypeSystemd,
		ServiceTypeSysV,
		ServiceTypeDocker,
	}

	expectedValues := []string{"systemd", "sysv", "docker"}

	for i, serviceType := range types {
		if string(serviceType) != expectedValues[i] {
			t.Errorf("Type constant %d mismatch: expected %s, got %s", i, expectedValues[i], string(serviceType))
		}
	}
}