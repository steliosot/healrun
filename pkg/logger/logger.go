package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	logDir      = ".healrun/logs"
	logFileBase = "install_"
)

var (
	logFilePath string
	once        sync.Once
	mu          sync.Mutex
)

// Init initializes the logging system
func Init() error {
	var initErr error

	once.Do(func() {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			initErr = fmt.Errorf("failed to get home directory: %w", err)
			return
		}

		logDirPath := filepath.Join(homeDir, logDir)
		if err := os.MkdirAll(logDirPath, 0755); err != nil {
			initErr = fmt.Errorf("failed to create log directory: %w", err)
			return
		}

		timestamp := time.Now().Format("20060102_150405")
		logFilePath = filepath.Join(logDirPath, logFileBase+timestamp+".log")
	})

	return initErr
}

// Write writes a log entry with timestamp
func Write(eventType, message string) {
	mu.Lock()
	defer mu.Unlock()

	if logFilePath == "" {
		return
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	logEntry := fmt.Sprintf("[%s] %s: %s\n", timestamp, eventType, message)

	f, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	f.WriteString(logEntry)
}

// Printf writes formatted output to stdout
func Printf(format string, args ...interface{}) {
	fmt.Printf(format, args...)
}

// Println writes a line to stdout
func Println(args ...interface{}) {
	fmt.Println(args...)
}

// Errorf writes formatted error to stderr
func Errorf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format, args...)
}

// Errorln writes error to stderr
func Errorln(args ...interface{}) {
	fmt.Fprintln(os.Stderr, args...)
}

// Debug writes debug output only if debug mode is enabled
func Debug(format string, args ...interface{}) {
	if IsDebug() {
		fmt.Fprintf(os.Stderr, "[DEBUG] "+format+"\n", args...)
	}
}

// Debugf writes formatted debug output only if debug mode is enabled
func Debugf(format string, args ...interface{}) {
	if IsDebug() {
		fmt.Fprintf(os.Stderr, "[DEBUG] "+format+"\n", args...)
	}
}

// IsDebug checks if debug mode is enabled via env var
func IsDebug() bool {
	return os.Getenv("HEALRUN_DEBUG") == "true"
}
