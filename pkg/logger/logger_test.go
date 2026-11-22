package logger

import (
	"bytes"
	"testing"
)

func TestLogFunctions(t *testing.T) {
	originalOutput := output

	defer func() { output = originalOutput }()

	var buf bytes.Buffer
	SetOutput(&buf)

	tests := []struct {
		name     string
		logFunc  func()
		expected string
	}{
		{
			name:     "Warnf",
			logFunc:  func() { Warnf("Test %s", "warning") },
			expected: "WARNING: Test warning\n",
		},
		{
			name:     "Error",
			logFunc:  func() { Error("Test", "error") },
			expected: "ERROR: Test error\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()
			tt.logFunc()

			if buf.String() != tt.expected {
				t.Errorf("output = %q, want %q", buf.String(), tt.expected)
			}
		})
	}
}

func TestCreateErrorf(t *testing.T) {
	err := CreateErrorf("test %s", "error")
	if err == nil {
		t.Fatal("CreateErrorf returned nil")
	}

	if err.Error() != "test error" {
		t.Errorf("Expected error message 'test error', got %q", err.Error())
	}
}
