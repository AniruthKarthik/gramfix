// Package log provides a lightweight structured logger for gramfix.
package log

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
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
	ts := time.Now().Format("15:04:05.000")
	line := fmt.Sprintf("[%s] %s [%s] %s", ts, levelNames[level], l.prefix, fmt.Sprintf(msg, args...))
	fmt.Fprintln(l.w, line)
}

// Debug logs a debug message.
func Debug(msg string, args ...any) { std.log(LevelDebug, msg, args...) }

// Info logs an informational message.
func Info(msg string, args ...any) { std.log(LevelInfo, msg, args...) }

// Warn logs a warning message.
func Warn(msg string, args ...any) { std.log(LevelWarn, msg, args...) }

// Error logs an error message.
func Error(msg string, args ...any) { std.log(LevelError, msg, args...) }

// LogFile opens or creates a log file inside ~/.local/share/gramfix/gramfix.log.
func LogFile() (*os.File, error) {
	dir := filepath.Join(os.Getenv("HOME"), ".local", "share", "gramfix")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, "gramfix.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	std.w = io.MultiWriter(os.Stderr, f)
	return f, nil
}
