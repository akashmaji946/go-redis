package common

// logger.go contains logging utilities for the Go-Redis server.
// It supports different log levels and formats log messages consistently
// across the application.

import (
	"log"
	"os"
)

var logger = NewLogger()

// Log levels
const (
	INFO_  = "INFO"
	WARN_  = "WARN"
	ERROR_ = "ERROR"
	DEBUG_ = "DEBUG"
)

// Logger is a custom logger with different log levels.
type Logger struct {
	infoLogger  *log.Logger
	warnLogger  *log.Logger
	errorLogger *log.Logger
	debugLogger *log.Logger
}

// NewLogger initializes and returns a new Logger instance.
func NewLogger() *Logger {
	return &Logger{
		infoLogger:  log.New(os.Stderr, "[INFO]  ", log.Ldate|log.Ltime),
		warnLogger:  log.New(os.Stderr, "[WARN]  ", log.Ldate|log.Ltime),
		errorLogger: log.New(os.Stderr, "[ERROR] ", log.Ldate|log.Ltime),
		debugLogger: log.New(os.Stderr, "[DEBUG] ", log.Ldate|log.Ltime),
	}
}

// Info logs an informational message.
func (l *Logger) Info(format string, v ...interface{}) {
	l.Printf("INFO", format, v...)
}

// Warn logs a warning message.
func (l *Logger) Warn(format string, v ...interface{}) {
	l.Printf("WARN", format, v...)
}

// Error logs an error message.
func (l *Logger) Error(format string, v ...interface{}) {
	l.Printf("ERROR", format, v...)
}

// Printf:
func (l *Logger) Printf(level string, format string, v ...interface{}) {
	switch level {
	case INFO_:
		l.infoLogger.Printf(format, v...) // v... unpacks the slice
	case WARN_:
		l.warnLogger.Printf(format, v...)
	case ERROR_:
		l.errorLogger.Printf(format, v...)
	case DEBUG_:
		l.debugLogger.Printf(format, v...)
	default:
		l.infoLogger.Printf(format, v...)
	}
}

// Println:
func (l *Logger) Println(level string, v ...interface{}) {
	switch level {
	case INFO_:
		l.infoLogger.Println(v...) // v... unpacks the slice
	case WARN_:
		l.warnLogger.Println(v...)
	case ERROR_:
		l.errorLogger.Println(v...)
	case DEBUG_:
		l.debugLogger.Println(v...)
	default:
		l.infoLogger.Println(v...)
	}
}
