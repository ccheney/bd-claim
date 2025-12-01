package infrastructure

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
)

// LogLevel represents the severity of a log entry.
type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
)

func (l LogLevel) String() string {
	switch l {
	case LogLevelDebug:
		return "DEBUG"
	case LogLevelInfo:
		return "INFO"
	case LogLevelWarn:
		return "WARN"
	case LogLevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// JSONLogger implements LoggerPort with JSON output.
type JSONLogger struct {
	out      io.Writer
	minLevel LogLevel
}

// NewJSONLogger creates a new JSONLogger.
func NewJSONLogger(minLevel LogLevel) *JSONLogger {
	return &JSONLogger{
		out:      os.Stderr,
		minLevel: minLevel,
	}
}

// SetOutput sets the output writer (for testing).
func (l *JSONLogger) SetOutput(w io.Writer) {
	l.out = w
}

func (l *JSONLogger) log(level LogLevel, msg string, fields map[string]interface{}) {
	if level < l.minLevel {
		return
	}

	entry := make(map[string]interface{})
	entry["timestamp"] = time.Now().Format(time.RFC3339Nano)
	entry["level"] = level.String()
	entry["message"] = msg

	for k, v := range fields {
		entry[k] = v
	}

	data, err := json.Marshal(entry)
	if err != nil {
		fmt.Fprintf(l.out, `{"error":"failed to marshal log entry: %s"}`+"\n", err.Error())
		return
	}
	fmt.Fprintln(l.out, string(data))
}

// Debug logs a debug message.
func (l *JSONLogger) Debug(msg string, fields map[string]interface{}) {
	l.log(LogLevelDebug, msg, fields)
}

// Info logs an info message.
func (l *JSONLogger) Info(msg string, fields map[string]interface{}) {
	l.log(LogLevelInfo, msg, fields)
}

// Warn logs a warning message.
func (l *JSONLogger) Warn(msg string, fields map[string]interface{}) {
	l.log(LogLevelWarn, msg, fields)
}

// Error logs an error message.
func (l *JSONLogger) Error(msg string, fields map[string]interface{}) {
	l.log(LogLevelError, msg, fields)
}

// NullLogger discards all log messages.
type NullLogger struct {
	// callCount tracks method calls for testing coverage
	callCount int
}

// NewNullLogger creates a new NullLogger.
func NewNullLogger() *NullLogger {
	return &NullLogger{}
}

// Debug discards the message.
func (l *NullLogger) Debug(msg string, fields map[string]interface{}) {
	l.callCount++
}

// Info discards the message.
func (l *NullLogger) Info(msg string, fields map[string]interface{}) {
	l.callCount++
}

// Warn discards the message.
func (l *NullLogger) Warn(msg string, fields map[string]interface{}) {
	l.callCount++
}

// Error discards the message.
func (l *NullLogger) Error(msg string, fields map[string]interface{}) {
	l.callCount++
}
