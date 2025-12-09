package logger

import (
	"log"
	"os"
)

// Logger wraps standard logger with additional functionality
type Logger struct {
	*log.Logger
}

// New creates a new logger instance
func New(prefix string) *Logger {
	return &Logger{
		Logger: log.New(os.Stdout, prefix, log.LstdFlags|log.Lshortfile),
	}
}

// Default returns a default logger
func Default() *Logger {
	return &Logger{
		Logger: log.Default(),
	}
}
