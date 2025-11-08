package logger

import (
	"fmt"
	"log"
	"os"
	"time"
)

// LogLevel represents the severity level of a log message
type LogLevel int

const (
	// DebugLevel is for detailed diagnostic messages
	DebugLevel LogLevel = iota
	// InfoLevel is for general informational messages
	InfoLevel
	// WarningLevel is for potentially harmful situations
	WarningLevel
	// ErrorLevel is for error conditions
	ErrorLevel
	// FatalLevel is for severe error conditions that cause program termination
	FatalLevel
)

var levelNames = map[LogLevel]string{
	DebugLevel:   "DEBUG",
	InfoLevel:    "INFO",
	WarningLevel: "WARNING",
	ErrorLevel:   "ERROR",
	FatalLevel:   "FATAL",
}

var currentLevel LogLevel
var logFile *os.File

// Init initializes the logger with the specified configuration
func Init(config LoggingConfig) {
	// Set log level
	switch config.Level {
	case "debug":
		currentLevel = DebugLevel
	case "info":
		currentLevel = InfoLevel
	case "warning":
		currentLevel = WarningLevel
	case "error":
		currentLevel = ErrorLevel
	case "fatal":
		currentLevel = FatalLevel
	default:
		currentLevel = InfoLevel
	}

	// Open log file if specified
	if config.File != "" {
		dir := "logs"
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			if err := os.Mkdir(dir, 0755); err != nil {
				log.Printf("Warning: Failed to create logs directory: %v", err)
			}
		}

		var err error
		logFile, err = os.OpenFile(config.File, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Printf("Warning: Failed to open log file: %v", err)
		}
	}
}

// logMessage logs a message with the specified level
func logMessage(level LogLevel, format string, args ...interface{}) {
	if level < currentLevel {
		return
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	message := fmt.Sprintf(format, args...)
	logEntry := fmt.Sprintf("[%s] [%s] %s\n", timestamp, levelNames[level], message)

	// Write to stdout
	fmt.Print(logEntry)

	// Write to file if configured
	if logFile != nil {
		logFile.WriteString(logEntry)
	}

	// Exit on fatal level
	if level == FatalLevel {
		os.Exit(1)
	}
}

// Debug logs a debug message
func Debug(format string, args ...interface{}) {
	logMessage(DebugLevel, format, args...)
}

// Info logs an info message
func Info(format string, args ...interface{}) {
	logMessage(InfoLevel, format, args...)
}

// Warning logs a warning message
func Warning(format string, args ...interface{}) {
	logMessage(WarningLevel, format, args...)
}

// Error logs an error message
func Error(format string, args ...interface{}) {
	logMessage(ErrorLevel, format, args...)
}

// Fatal logs a fatal message and exits the program
func Fatal(format string, args ...interface{}) {
	logMessage(FatalLevel, format, args...)
}