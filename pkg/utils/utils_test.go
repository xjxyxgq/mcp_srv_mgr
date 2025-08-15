package utils

import (
	"testing"
	"time"
)

func TestParseInt(t *testing.T) {
	tests := []struct {
		input    string
		expected int
		hasError bool
	}{
		{"123", 123, false},
		{"0", 0, false},
		{"-456", -456, false},
		{"abc", 0, true},
		{"", 0, true},
		{"123.45", 0, true},
	}

	for _, test := range tests {
		result, err := ParseInt(test.input)
		
		if test.hasError {
			if err == nil {
				t.Errorf("Expected error for input %q, but got none", test.input)
			}
		} else {
			if err != nil {
				t.Errorf("Unexpected error for input %q: %v", test.input, err)
			}
			if result != test.expected {
				t.Errorf("For input %q, expected %d, got %d", test.input, test.expected, result)
			}
		}
	}
}

func TestGetCurrentTimestamp(t *testing.T) {
	timestamp := GetCurrentTimestamp()
	
	// 验证时间戳格式
	_, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		t.Errorf("Invalid timestamp format: %s, error: %v", timestamp, err)
	}
	
	// 验证时间戳是最近的（在测试运行的1秒内）
	parsedTime, _ := time.Parse(time.RFC3339, timestamp)
	timeDiff := time.Since(parsedTime)
	if timeDiff > time.Second {
		t.Errorf("Timestamp is too old: %v", timeDiff)
	}
}