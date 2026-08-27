package logger

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
)

// JSONFormatter outputs log entries in JSON format for structured log processing.
type JSONFormatter struct {
	output io.Writer
	level  Level
}

// NewJSONFormatter creates a new JSON-formatted logger output.
func NewJSONFormatter(output io.Writer, level Level) *JSONFormatter {
	return &JSONFormatter{
		output: output,
		level:  level,
	}
}

// FormatEntry creates a JSON-formatted log entry.
func (f *JSONFormatter) FormatEntry(level Level, msg string, fields map[string]interface{}) []byte {
	entry := map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
		"level":     level.String(),
		"message":   msg,
	}

	for k, v := range fields {
		entry[k] = v
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return []byte(fmt.Sprintf(`{"level":"ERROR","message":"failed to marshal log entry: %s"}`, err.Error()))
	}
	data = append(data, '\n')
	return data
}

// Write writes a JSON-formatted entry to the output.
func (f *JSONFormatter) Write(level Level, msg string, fields map[string]interface{}) {
	if level < f.level {
		return
	}
	data := f.FormatEntry(level, msg, fields)
	_, _ = f.output.Write(data)
}

// JSONLogger is a logger that outputs in JSON format.
type JSONLogger struct {
	*Logger
	formatter *JSONFormatter
}

// NewJSONLogger creates a JSON-output logger.
func NewJSONLogger(output io.Writer, level Level) *JSONLogger {
	logger := New(output, level, "")
	return &JSONLogger{
		Logger:    logger,
		formatter: NewJSONFormatter(output, level),
	}
}

// DebugfJSON logs a JSON entry at debug level.
func (l *JSONLogger) DebugfJSON(format string, fields map[string]interface{}, args ...interface{}) {
	l.formatter.Write(LevelDebug, fmt.Sprintf(format, args...), fields)
}

// InfofJSON logs a JSON entry at info level.
func (l *JSONLogger) InfofJSON(format string, fields map[string]interface{}, args ...interface{}) {
	l.formatter.Write(LevelInfo, fmt.Sprintf(format, args...), fields)
}

// WarnfJSON logs a JSON entry at warn level.
func (l *JSONLogger) WarnfJSON(format string, fields map[string]interface{}, args ...interface{}) {
	l.formatter.Write(LevelWarn, fmt.Sprintf(format, args...), fields)
}

// ErrorfJSON logs a JSON entry at error level.
func (l *JSONLogger) ErrorfJSON(format string, fields map[string]interface{}, args ...interface{}) {
	l.formatter.Write(LevelError, fmt.Sprintf(format, args...), fields)
}

// FileLogger writes logs to a file.
type FileLogger struct {
	logger   *Logger
	file     *os.File
	filePath string
}

// NewFileLogger creates a logger that writes to a file.
func NewFileLogger(filePath string, level Level) (*FileLogger, error) {
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}
	l := New(f, level, "")
	return &FileLogger{
		logger:   l,
		file:     f,
		filePath: filePath,
	}, nil
}

// Logger returns the underlying Logger.
func (f *FileLogger) Logger() *Logger {
	return f.logger
}

// Close closes the log file.
func (f *FileLogger) Close() error {
	return f.file.Close()
}
