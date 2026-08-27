package logger

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
)

func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	case LevelFatal:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

func ParseLevel(s string) Level {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "DEBUG":
		return LevelDebug
	case "INFO":
		return LevelInfo
	case "WARN", "WARNING":
		return LevelWarn
	case "ERROR", "ERR":
		return LevelError
	case "FATAL":
		return LevelFatal
	default:
		return LevelInfo
	}
}

type Logger struct {
	mu           sync.Mutex
	output       io.Writer
	level        Level
	prefix       string
	fields       map[string]interface{}
	buf          bytes.Buffer
	flags        int
	maxEntrySize int
}

const (
	FlagDate = 1 << iota
	FlagTime
	FlagMicroseconds
	FlagFile
	FlagLine
	FlagFunc
)

var DefaultLogger = New(os.Stdout, LevelInfo, "")

func New(output io.Writer, level Level, prefix string) *Logger {
	return &Logger{
		output:       output,
		level:        level,
		prefix:       prefix,
		fields:       make(map[string]interface{}),
		flags:        FlagDate | FlagTime | FlagMicroseconds | FlagFile | FlagLine,
		maxEntrySize: defaultMaxEntrySize,
	}
}

func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

func (l *Logger) GetLevel() Level {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.level
}

func (l *Logger) SetMaxEntrySize(size int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.maxEntrySize = size
}

func (l *Logger) GetMaxEntrySize() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.maxEntrySize
}

func (l *Logger) WithField(key string, value interface{}) *Logger {
	newLogger := &Logger{
		output:       l.output,
		level:        l.level,
		prefix:       l.prefix,
		fields:       make(map[string]interface{}),
		flags:        l.flags,
		maxEntrySize: l.maxEntrySize,
	}
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}
	newLogger.fields[key] = value
	return newLogger
}

func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	newLogger := &Logger{
		output:       l.output,
		level:        l.level,
		prefix:       l.prefix,
		fields:       make(map[string]interface{}),
		flags:        l.flags,
		maxEntrySize: l.maxEntrySize,
	}
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}
	for k, v := range fields {
		newLogger.fields[k] = v
	}
	return newLogger
}

func (l *Logger) SetOutput(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.output = w
}

func (l *Logger) Debug(args ...interface{}) {
	l.log(LevelDebug, 1, args...)
}

func (l *Logger) Debugf(format string, args ...interface{}) {
	l.logf(LevelDebug, 1, format, args...)
}

func (l *Logger) Info(args ...interface{}) {
	l.log(LevelInfo, 1, args...)
}

func (l *Logger) Infof(format string, args ...interface{}) {
	l.logf(LevelInfo, 1, format, args...)
}

func (l *Logger) Warn(args ...interface{}) {
	l.log(LevelWarn, 1, args...)
}

func (l *Logger) Warnf(format string, args ...interface{}) {
	l.logf(LevelWarn, 1, format, args...)
}

func (l *Logger) Error(args ...interface{}) {
	l.log(LevelError, 1, args...)
}

func (l *Logger) Errorf(format string, args ...interface{}) {
	l.logf(LevelError, 1, format, args...)
}

func (l *Logger) Fatal(args ...interface{}) {
	l.log(LevelFatal, 1, args...)
	os.Exit(1)
}

func (l *Logger) Fatalf(format string, args ...interface{}) {
	l.logf(LevelFatal, 1, format, args...)
	os.Exit(1)
}

func DebugfDefault(format string, args ...interface{}) {
	DefaultLogger.logf(LevelDebug, 1, format, args...)
}

func InfofDefault(format string, args ...interface{}) {
	DefaultLogger.logf(LevelInfo, 1, format, args...)
}

func WarnfDefault(format string, args ...interface{}) {
	DefaultLogger.logf(LevelWarn, 1, format, args...)
}

func ErrorfDefault(format string, args ...interface{}) {
	DefaultLogger.logf(LevelError, 1, format, args...)
}

func (l *Logger) log(level Level, depth int, args ...interface{}) {
	if level < l.level {
		return
	}
	msg := fmt.Sprint(args...)
	l.outputEntry(level, depth+1, msg)
}

func (l *Logger) logf(level Level, depth int, format string, args ...interface{}) {
	if level < l.level {
		return
	}
	msg := fmt.Sprintf(format, args...)
	l.outputEntry(level, depth+1, msg)
}

func (l *Logger) outputEntry(level Level, depth int, msg string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	l.buf.Reset()

	if l.flags&(FlagDate|FlagTime|FlagMicroseconds) != 0 {
		year, month, day := now.Date()
		hour, minute, second := now.Clock()
		_, _ = fmt.Fprintf(&l.buf, "%04d/%02d/%02d %02d:%02d:%02d",
			year, month, day, hour, minute, second)
		if l.flags&FlagMicroseconds != 0 {
			_, _ = fmt.Fprintf(&l.buf, ".%06d", now.Nanosecond()/1000)
		}
		l.buf.WriteByte(' ')
	}

	levelStr := level.String()
	l.buf.WriteByte('[')
	l.buf.WriteString(levelStr)
	l.buf.WriteByte(']')
	l.buf.WriteByte(' ')

	if l.flags&(FlagFile|FlagLine|FlagFunc) != 0 {
		if _, file, line, ok := runtime.Caller(depth); ok {
			if l.flags&FlagFile != 0 {
				l.buf.WriteByte('(')
				shortFile := file
				if len(shortFile) > 20 {
					shortFile = "..." + shortFile[len(shortFile)-17:]
				}
				l.buf.WriteString(shortFile)
				if l.flags&FlagLine != 0 {
					l.buf.WriteByte(':')
					_, _ = fmt.Fprintf(&l.buf, "%d", line)
				}
				l.buf.WriteByte(')')
				l.buf.WriteByte(' ')
			}
		}
	}

	if l.prefix != "" {
		l.buf.WriteByte('[')
		l.buf.WriteString(l.prefix)
		l.buf.WriteByte(']')
		l.buf.WriteByte(' ')
	}

	if len(l.fields) > 0 {
		l.buf.WriteByte('{')
		first := true
		for k, v := range l.fields {
			if !first {
				l.buf.WriteByte(',')
			}
			first = false
			l.buf.WriteString(k)
			l.buf.WriteByte('=')
			l.buf.WriteString(fmt.Sprint(v))
		}
		l.buf.WriteByte('}')
		l.buf.WriteByte(' ')
	}

	l.buf.WriteString(msg)
	l.buf.WriteByte('\n')

	if l.buf.Len() > l.maxEntrySize {
		truncated := l.truncateOutput(l.buf.Bytes())
		_, _ = l.output.Write(truncated)
	} else {
		_, _ = l.output.Write(l.buf.Bytes())
	}
}

func (l *Logger) truncateOutput(data []byte) []byte {
	if len(data) <= l.maxEntrySize {
		return data
	}

	headerSize := l.maxEntrySize / 2
	copyLen := headerSize
	if copyLen > len(data) {
		copyLen = len(data)
	}

	result := make([]byte, copyLen+len("...(truncated)\n"))
	copy(result, data[:copyLen])
	copy(result[copyLen:], "...(truncated)\n")
	return result
}

func (l *Logger) InfofJSON(format string, fields map[string]interface{}, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	l.writeJSON(LevelInfo, msg, fields)
}

func (l *Logger) WarnfJSON(format string, fields map[string]interface{}, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	l.writeJSON(LevelWarn, msg, fields)
}

func (l *Logger) ErrorfJSON(format string, fields map[string]interface{}, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	l.writeJSON(LevelError, msg, fields)
}

func (l *Logger) DebugfJSON(format string, fields map[string]interface{}, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	l.writeJSON(LevelDebug, msg, fields)
}

func (l *Logger) writeJSON(level Level, msg string, fields map[string]interface{}) {
	if level < l.level {
		return
	}
	formatter := NewJSONFormatter(l.output, level)
	formatter.SetMaxEntrySize(l.maxEntrySize)
	data := formatter.FormatEntry(level, msg, fields)
	_, _ = l.output.Write(data)
}
