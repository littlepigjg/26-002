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

// Level represents the severity level of a log message.
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

// ParseLevel converts a string to a Level.
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

// Logger is the core logging structure.
type Logger struct {
	mu     sync.Mutex
	output io.Writer
	level  Level
	prefix string
	fields map[string]interface{}
	buf    bytes.Buffer
	flags  int
}

const (
	FlagDate = 1 << iota
	FlagTime
	FlagMicroseconds
	FlagFile
	FlagLine
	FlagFunc
)

// DefaultLogger is the package-level default logger instance.
var DefaultLogger = New(os.Stdout, LevelInfo, "")

// New creates a new Logger instance.
func New(output io.Writer, level Level, prefix string) *Logger {
	return &Logger{
		output: output,
		level:  level,
		prefix: prefix,
		fields: make(map[string]interface{}),
		flags:  FlagDate | FlagTime | FlagMicroseconds | FlagFile | FlagLine,
	}
}

// SetLevel changes the minimum log level.
func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// GetLevel returns the current log level.
func (l *Logger) GetLevel() Level {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.level
}

// WithField returns a new Logger with an additional field.
func (l *Logger) WithField(key string, value interface{}) *Logger {
	newLogger := &Logger{
		output: l.output,
		level:  l.level,
		prefix: l.prefix,
		fields: make(map[string]interface{}),
		flags:  l.flags,
	}
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}
	newLogger.fields[key] = value
	return newLogger
}

// WithFields returns a new Logger with additional fields.
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	newLogger := &Logger{
		output: l.output,
		level:  l.level,
		prefix: l.prefix,
		fields: make(map[string]interface{}),
		flags:  l.flags,
	}
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}
	for k, v := range fields {
		newLogger.fields[k] = v
	}
	return newLogger
}

// SetOutput changes the output writer.
func (l *Logger) SetOutput(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.output = w
}

// Debug logs a message at DEBUG level.
func (l *Logger) Debug(args ...interface{}) {
	l.log(LevelDebug, 1, args...)
}

// Debugf logs a formatted message at DEBUG level.
func (l *Logger) Debugf(format string, args ...interface{}) {
	l.logf(LevelDebug, 1, format, args...)
}

// Info logs a message at INFO level.
func (l *Logger) Info(args ...interface{}) {
	l.log(LevelInfo, 1, args...)
}

// Infof logs a formatted message at INFO level.
func (l *Logger) Infof(format string, args ...interface{}) {
	l.logf(LevelInfo, 1, format, args...)
}

// Warn logs a message at WARN level.
func (l *Logger) Warn(args ...interface{}) {
	l.log(LevelWarn, 1, args...)
}

// Warnf logs a formatted message at WARN level.
func (l *Logger) Warnf(format string, args ...interface{}) {
	l.logf(LevelWarn, 1, format, args...)
}

// Error logs a message at ERROR level.
func (l *Logger) Error(args ...interface{}) {
	l.log(LevelError, 1, args...)
}

// Errorf logs a formatted message at ERROR level.
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.logf(LevelError, 1, format, args...)
}

// Fatal logs a message at FATAL level and exits.
func (l *Logger) Fatal(args ...interface{}) {
	l.log(LevelFatal, 1, args...)
	os.Exit(1)
}

// Fatalf logs a formatted message at FATAL level and exits.
func (l *Logger) Fatalf(format string, args ...interface{}) {
	l.logf(LevelFatal, 1, format, args...)
	os.Exit(1)
}

// DebugfDefault logs at DEBUG level on the default logger.
func DebugfDefault(format string, args ...interface{}) {
	DefaultLogger.logf(LevelDebug, 1, format, args...)
}

// InfofDefault logs at INFO level on the default logger.
func InfofDefault(format string, args ...interface{}) {
	DefaultLogger.logf(LevelInfo, 1, format, args...)
}

// WarnfDefault logs at WARN level on the default logger.
func WarnfDefault(format string, args ...interface{}) {
	DefaultLogger.logf(LevelWarn, 1, format, args...)
}

// ErrorfDefault logs at ERROR level on the default logger.
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

	// Append timestamp
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

	// Append level
	levelStr := level.String()
	l.buf.WriteByte('[')
	l.buf.WriteString(levelStr)
	l.buf.WriteByte(']')
	l.buf.WriteByte(' ')

	// Append caller info
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

	// Append prefix
	if l.prefix != "" {
		l.buf.WriteByte('[')
		l.buf.WriteString(l.prefix)
		l.buf.WriteByte(']')
		l.buf.WriteByte(' ')
	}

	// Append fields
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

	// Append message
	l.buf.WriteString(msg)
	l.buf.WriteByte('\n')

	_, _ = l.output.Write(l.buf.Bytes())
}
