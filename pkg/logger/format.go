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
	// Cap a large message before marshalling so it can't dominate the entry.
	truncatedMsg := msg
	if len(truncatedMsg) > f.maxEntrySize/2 {
		truncatedMsg = msg[:f.maxEntrySize/2] + "...(truncated)"
	}

	// Start with a generous per-field budget and shrink it until the
	// serialized entry fits. Re-marshalling on each iteration keeps the
	// output valid JSON regardless of how many fields there are.
	fieldBudget := f.maxEntrySize / 4
	if fieldBudget < 1 {
		fieldBudget = 1
	}

	var data []byte
	for {
		truncatedFields := truncateFieldValues(cloneFields(fields), fieldBudget)
		entry := map[string]interface{}{
			"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
			"level":     level.String(),
			"message":   truncatedMsg,
		}
		for k, v := range truncatedFields {
			entry[k] = v
		}

		var err error
		data, err = json.Marshal(entry)
		if err != nil {
			return []byte(fmt.Sprintf(`{"level":"ERROR","message":"failed to marshal log entry: %s"}`, err.Error()))
		}

		if len(data) <= f.maxEntrySize || fieldBudget <= 1 {
			break
		}
		// Didn't fit; halve the per-field budget and try again.
		fieldBudget /= 2
		if fieldBudget < 1 {
			fieldBudget = 1
		}
	}

	if len(data) > f.maxEntrySize {
		// Final safety net for tiny budgets where even minimal fields
		// overflow (or non-string field values we can't shrink). Emit a
		// valid, self-contained truncated entry instead of slicing bytes.
		data = f.fallbackTruncatedEntry(level)
	}

	data = append(data, '\n')
	return data
}

// fallbackTruncatedEntry is the last-resort entry when the per-field shrinking
// loop could not bring the entry under budget. It is guaranteed to be valid
// JSON and to fit within maxEntrySize when maxEntrySize is at least the
// length of the minimal stub.
func (f *JSONFormatter) fallbackTruncatedEntry(level Level) []byte {
	stub := []byte(fmt.Sprintf(`{"level":"%s","message":"...(truncated)"}`, level.String()))
	if len(stub) <= f.maxEntrySize {
		return stub
	}
	// Budget too small for even the stub; truncate the stub itself. This can
	// produce invalid JSON, but only when maxEntrySize is smaller than a
	// minimal log line, which is not a supported configuration.
	return stub[:f.maxEntrySize]
}

func cloneFields(fields map[string]interface{}) map[string]interface{} {
	clone := make(map[string]interface{}, len(fields)+3)
	for k, v := range fields {
		clone[k] = v
	}
	return clone
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
