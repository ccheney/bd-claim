package infrastructure

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestLogLevel_String(t *testing.T) {
	tests := []struct {
		level    LogLevel
		expected string
	}{
		{LogLevelDebug, "DEBUG"},
		{LogLevelInfo, "INFO"},
		{LogLevelWarn, "WARN"},
		{LogLevelError, "ERROR"},
		{LogLevel(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if tt.level.String() != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, tt.level.String())
			}
		})
	}
}

func TestJSONLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := NewJSONLogger(LogLevelDebug)
	logger.SetOutput(&buf)

	logger.Info("test message", map[string]interface{}{"key": "value"})

	output := buf.String()
	if !strings.Contains(output, "test message") {
		t.Errorf("expected output to contain 'test message', got '%s'", output)
	}
	if !strings.Contains(output, "INFO") {
		t.Errorf("expected output to contain 'INFO', got '%s'", output)
	}

	// Parse JSON
	var entry map[string]interface{}
	if err := json.Unmarshal([]byte(output), &entry); err != nil {
		t.Fatalf("failed to parse log entry: %v", err)
	}
	if entry["key"] != "value" {
		t.Errorf("expected key 'value', got '%v'", entry["key"])
	}
}

func TestJSONLogger_Levels(t *testing.T) {
	tests := []struct {
		name      string
		minLevel  LogLevel
		logLevel  LogLevel
		logFunc   func(*JSONLogger, string, map[string]interface{})
		shouldLog bool
	}{
		{"debug at debug level", LogLevelDebug, LogLevelDebug, (*JSONLogger).Debug, true},
		{"info at debug level", LogLevelDebug, LogLevelInfo, (*JSONLogger).Info, true},
		{"debug at info level", LogLevelInfo, LogLevelDebug, (*JSONLogger).Debug, false},
		{"warn at info level", LogLevelInfo, LogLevelWarn, (*JSONLogger).Warn, true},
		{"error at error level", LogLevelError, LogLevelError, (*JSONLogger).Error, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := NewJSONLogger(tt.minLevel)
			logger.SetOutput(&buf)

			tt.logFunc(logger, "test", nil)

			hasOutput := buf.Len() > 0
			if hasOutput != tt.shouldLog {
				t.Errorf("expected shouldLog=%v, got hasOutput=%v", tt.shouldLog, hasOutput)
			}
		})
	}
}

func TestJSONLogger_AllMethods(t *testing.T) {
	var buf bytes.Buffer
	logger := NewJSONLogger(LogLevelDebug)
	logger.SetOutput(&buf)

	logger.Debug("debug msg", nil)
	logger.Info("info msg", nil)
	logger.Warn("warn msg", nil)
	logger.Error("error msg", nil)

	output := buf.String()
	if !strings.Contains(output, "DEBUG") {
		t.Error("expected DEBUG in output")
	}
	if !strings.Contains(output, "INFO") {
		t.Error("expected INFO in output")
	}
	if !strings.Contains(output, "WARN") {
		t.Error("expected WARN in output")
	}
	if !strings.Contains(output, "ERROR") {
		t.Error("expected ERROR in output")
	}
}

func TestNullLogger(t *testing.T) {
	logger := NewNullLogger()

	// Should not panic
	logger.Debug("msg", nil)
	logger.Info("msg", nil)
	logger.Warn("msg", nil)
	logger.Error("msg", nil)
}

func TestJSONLogger_MarshalError(t *testing.T) {
	var buf bytes.Buffer
	logger := NewJSONLogger(LogLevelDebug)
	logger.SetOutput(&buf)

	// Create a value that can't be marshaled (channel)
	ch := make(chan int)
	logger.Info("test", map[string]interface{}{"bad": ch})

	output := buf.String()
	if !strings.Contains(output, "failed to marshal") {
		t.Errorf("expected marshal error message, got '%s'", output)
	}
}
