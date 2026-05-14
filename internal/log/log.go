// Package log provides a lightweight structured logger for gramfix.
package log

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Level represents the log severity.
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

var levelNames = map[Level]string{
	LevelDebug: "DEBUG",
	LevelInfo:  "INFO",
	LevelWarn:  "WARN",
	LevelError:  "ERROR",
}

// Logger is a simple structured logger.
type Logger struct {
	level  Level
	w      io.Writer
	prefix string
}

var std = &Logger{level: LevelInfo, w: os.Stderr, prefix: "gramfix"}

// SetLevel sets the minimum log level.
func SetLevel(l Level) { std.level = l }

// SetDebug enables debug-level logging.
func SetDebug(enabled bool) {
	if enabled {
		std.level = LevelDebug
	} else {
		std.level = LevelInfo
	}
}

func (l *Logger) log(level Level, msg string, args ...any) {
	if level < l.level {
		return
	}
	ts := time.Now().Format("2006-01-02 15:04:05")
	line := fmt.Sprintf("[%s] %s: %s", ts, levelNames[level], fmt.Sprintf(msg, args...))
	fmt.Fprintln(l.w, line)
}

// Audit logs a correction in a structured format: date, time, sentence, method, corrected
func Audit(original, method, corrected string) {
	now := time.Now()
	date := now.Format("2006-01-02")
	timeStr := now.Format("15:04:05")

	// Escape quotes for CSV-like safety
	origEsc := strings.ReplaceAll(original, "\"", "\"\"")
	corrEsc := strings.ReplaceAll(corrected, "\"", "\"\"")

	line := fmt.Sprintf("%s, %s, \"%s\", %s, \"%s\"", date, timeStr, origEsc, method, corrEsc)
	if std.w != nil {
		fmt.Fprintln(std.w, line)
	}
}

// Debug logs a debug message.
func Debug(msg string, args ...any) { std.log(LevelDebug, msg, args...) }

// Info logs an informational message.
func Info(msg string, args ...any) { std.log(LevelInfo, msg, args...) }

// Warn logs a warning message.
func Warn(msg string, args ...any) { std.log(LevelWarn, msg, args...) }

// Error logs an error message.
func Error(msg string, args ...any) { std.log(LevelError, msg, args...) }

// LogDir returns the directory where logs are stored.
func LogDir() string {
	return filepath.Join(os.Getenv("HOME"), ".local", "share", "gramfix")
}

// LogFile opens or creates a log file inside ~/.local/share/gramfix/gramfix.log.
func LogFile() (*os.File, error) {
	dir := LogDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, "gramfix.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	// For audit simplicity, we'll write to the file only if requested, 
	// otherwise stderr is the default for general logs.
	std.w = io.MultiWriter(os.Stderr, f)
	return f, nil
}
