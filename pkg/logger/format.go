package logger

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

const defaultMaxEntrySize = 4096

type JSONFormatter struct {
	output       io.Writer
	level        Level
	maxEntrySize int
}

func NewJSONFormatter(output io.Writer, level Level) *JSONFormatter {
	return &JSONFormatter{
		output:       output,
		level:        level,
		maxEntrySize: defaultMaxEntrySize,
	}
}

func (f *JSONFormatter) SetMaxEntrySize(size int) {
	f.maxEntrySize = size
}

func (f *JSONFormatter) GetMaxEntrySize() int {
	return f.maxEntrySize
}

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

	if len(data) > f.maxEntrySize {
		data = f.applyMaxSize(data, level, msg, fields)
	}

	data = append(data, '\n')
	return data
}

func (f *JSONFormatter) applyMaxSize(data []byte, level Level, msg string, fields map[string]interface{}) []byte {
	if len(msg) > f.maxEntrySize/2 {
		truncatedMsg := msg[:f.maxEntrySize/2] + "...(truncated)"
		entry := map[string]interface{}{
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"level":     level.String(),
			"message":   truncatedMsg,
		}
		for k, v := range fields {
			entry[k] = v
		}
		result, err := json.Marshal(entry)
		if err != nil {
			return data[:f.maxEntrySize]
		}
		return result
	}

	return data
}

func (f *JSONFormatter) Write(level Level, msg string, fields map[string]interface{}) {
	if level < f.level {
		return
	}
	data := f.FormatEntry(level, msg, fields)
	_, _ = f.output.Write(data)
}

type JSONLogger struct {
	*Logger
	formatter    *JSONFormatter
	maxEntrySize int
}

func NewJSONLogger(output io.Writer, level Level) *JSONLogger {
	logger := New(output, level, "")
	return &JSONLogger{
		Logger:       logger,
		formatter:    NewJSONFormatter(output, level),
		maxEntrySize: defaultMaxEntrySize,
	}
}

func (l *JSONLogger) SetMaxEntrySize(size int) {
	l.maxEntrySize = size
	l.formatter.SetMaxEntrySize(size)
	l.Logger.SetMaxEntrySize(size)
}

func (l *JSONLogger) DebugfJSON(format string, fields map[string]interface{}, args ...interface{}) {
	l.formatter.Write(LevelDebug, fmt.Sprintf(format, args...), fields)
}

func (l *JSONLogger) InfofJSON(format string, fields map[string]interface{}, args ...interface{}) {
	l.formatter.Write(LevelInfo, fmt.Sprintf(format, args...), fields)
}

func (l *JSONLogger) WarnfJSON(format string, fields map[string]interface{}, args ...interface{}) {
	l.formatter.Write(LevelWarn, fmt.Sprintf(format, args...), fields)
}

func (l *JSONLogger) ErrorfJSON(format string, fields map[string]interface{}, args ...interface{}) {
	l.formatter.Write(LevelError, fmt.Sprintf(format, args...), fields)
}

type FileLogger struct {
	logger   *Logger
	file     *os.File
	filePath string
}

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

func (f *FileLogger) Logger() *Logger {
	return f.logger
}

func (f *FileLogger) Close() error {
	return f.file.Close()
}

func truncateFieldValues(entry map[string]interface{}, maxFieldSize int) map[string]interface{} {
	for k, v := range entry {
		if s, ok := v.(string); ok && len(s) > maxFieldSize {
			entry[k] = s[:maxFieldSize] + "...(truncated)"
		}
	}
	return entry
}

var _ = strings.TrimSpace
