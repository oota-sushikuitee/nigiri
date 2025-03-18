package logger

import (
	"bytes"
	"strings"
	"testing"
)

// TestSetOutput verifies that SetOutput correctly changes where logs are written
func TestSetOutput(t *testing.T) {
	// Save original output
	originalOutput := defaultOutput
	defer func() { defaultOutput = originalOutput }()

	// Create a buffer to capture output
	var buf bytes.Buffer
	SetOutput(&buf)

	// Verify output changed
	if defaultOutput != &buf {
		t.Errorf("SetOutput() did not change output")
	}

	// Verify logs are written to buffer
	Info("test message")
	if !strings.Contains(buf.String(), "test message") {
		t.Errorf("Log output not written to buffer: %q", buf.String())
	}
}

// TestSetLevel verifies that SetLevel correctly filters logs by level
func TestSetLevel(t *testing.T) {
	// Save original level
	originalLevel := defaultLevel
	defer func() { defaultLevel = originalLevel }()

	tests := []struct {
		level          LogLevel
		shouldLogDebug bool
		shouldLogInfo  bool
		shouldLogWarn  bool
		shouldLogError bool
	}{
		{DebugLevel, true, true, true, true},
		{InfoLevel, false, true, true, true},
		{WarnLevel, false, false, true, true},
		{ErrorLevel, false, false, false, true},
		{FatalLevel, false, false, false, false}, // Fatal would exit, so we don't test actual output
	}

	for _, tt := range tests {
		t.Run(levelToString(tt.level), func(t *testing.T) {
			var buf bytes.Buffer
			SetOutput(&buf)
			SetLevel(tt.level)

			Debug("debug message")
			if tt.shouldLogDebug && !strings.Contains(buf.String(), "debug message") {
				t.Errorf("Debug message should be logged at level %s", levelToString(tt.level))
			} else if !tt.shouldLogDebug && strings.Contains(buf.String(), "debug message") {
				t.Errorf("Debug message should not be logged at level %s", levelToString(tt.level))
			}

			buf.Reset()
			Info("info message")
			if tt.shouldLogInfo && !strings.Contains(buf.String(), "info message") {
				t.Errorf("Info message should be logged at level %s", levelToString(tt.level))
			} else if !tt.shouldLogInfo && strings.Contains(buf.String(), "info message") {
				t.Errorf("Info message should not be logged at level %s", levelToString(tt.level))
			}

			buf.Reset()
			Warn("warn message")
			if tt.shouldLogWarn && !strings.Contains(buf.String(), "warn message") {
				t.Errorf("Warn message should be logged at level %s", levelToString(tt.level))
			} else if !tt.shouldLogWarn && strings.Contains(buf.String(), "warn message") {
				t.Errorf("Warn message should not be logged at level %s", levelToString(tt.level))
			}

			buf.Reset()
			Error("error message")
			if tt.shouldLogError && !strings.Contains(buf.String(), "error message") {
				t.Errorf("Error message should be logged at level %s", levelToString(tt.level))
			} else if !tt.shouldLogError && strings.Contains(buf.String(), "error message") {
				t.Errorf("Error message should not be logged at level %s", levelToString(tt.level))
			}
		})
	}
}

// TestLogFunctions verifies formatted log functions work correctly
func TestLogFunctions(t *testing.T) {
	// Save original settings
	originalLevel := defaultLevel
	originalOutput := defaultOutput
	defer func() {
		defaultLevel = originalLevel
		defaultOutput = originalOutput
	}()

	SetLevel(DebugLevel)
	var buf bytes.Buffer
	SetOutput(&buf)

	tests := []struct {
		name     string
		logFunc  func()
		expected string
	}{
		{
			name:     "Debugf",
			logFunc:  func() { Debugf("Test %s", "debug") },
			expected: "DEBUG: Test debug",
		},
		{
			name:     "Infof",
			logFunc:  func() { Infof("Test %s", "info") },
			expected: "Test info",
		},
		{
			name:     "Warnf",
			logFunc:  func() { Warnf("Test %s", "warning") },
			expected: "WARNING: Test warning",
		},
		{
			name:     "Errorf",
			logFunc:  func() { Errorf("Test %s", "error") },
			expected: "ERROR: Test error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()
			tt.logFunc()
			if !strings.Contains(buf.String(), tt.expected) {
				t.Errorf("Expected log to contain %q, got %q", tt.expected, buf.String())
			}
		})
	}
}

// TestCreateErrorf verifies the error creation utility function
func TestCreateErrorf(t *testing.T) {
	err := CreateErrorf("test %s", "error")
	if err == nil {
		t.Error("CreateErrorf returned nil")
	}
	if err.Error() != "test error" {
		t.Errorf("Expected error message 'test error', got %q", err.Error())
	}
}

// Helper function to convert LogLevel to string for test naming
func levelToString(level LogLevel) string {
	switch level {
	case DebugLevel:
		return "Debug"
	case InfoLevel:
		return "Info"
	case WarnLevel:
		return "Warn"
	case ErrorLevel:
		return "Error"
	case FatalLevel:
		return "Fatal"
	default:
		return "Unknown"
	}
}

// TestReadInput verifies the ReadInput function
// Note: This test is limited because it requires stdin input
func TestReadInput(t *testing.T) {
	// Skip this test since it's difficult to mock stdin properly in this simple test setup
	t.Skip("Skipping ReadInput test due to stdin mocking complexity")

	// Alternatively, we could implement a more sophisticated mocking approach if needed
	// But for now, we'll skip it as this function simply wraps fmt.Scanln
}

// Test Fatal functions indirectly (since they call os.Exit)
func TestFatalFunctions(t *testing.T) {
	// We can't directly test functions that call os.Exit
	// So we just verify they exist and have correct signatures

	// This is just a placeholder to remind that these functions should be
	// tested in a more comprehensive way if critical (e.g., by using
	// a custom exit function that can be mocked in tests)

	// For now we just make sure the code compiles
	originalLevel := defaultLevel
	originalOutput := defaultOutput
	defer func() {
		defaultLevel = originalLevel
		defaultOutput = originalOutput
	}()

	// Set level to a value that won't trigger actual exit
	SetLevel(FatalLevel + 1)
	var buf bytes.Buffer
	SetOutput(&buf)

	Fatal("This shouldn't actually exit")
	Fatalf("This %s shouldn't actually exit", "also")
}
