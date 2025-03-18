// Package logger provides unified logging functionality for the nigiri CLI application
package logger

import (
	"fmt"
	"io"
	"os"
)

// LogLevel represents the severity of a log message
type LogLevel int

const (
	// DebugLevel logs verbose development info
	DebugLevel LogLevel = iota
	// InfoLevel logs general information
	InfoLevel
	// WarnLevel logs warnings that might cause issues
	WarnLevel
	// ErrorLevel logs errors that prevented an operation
	ErrorLevel
	// FatalLevel logs critical errors and exits the application
	FatalLevel
)

var (
	// Default output is stderr
	defaultOutput io.Writer = os.Stderr
	// Default log level is Info
	defaultLevel = InfoLevel
	// Whether to include log level prefix in output
	showPrefix = true
)

// SetOutput changes the output destination for the logger
func SetOutput(w io.Writer) {
	defaultOutput = w
}

// SetLevel changes the minimum log level that will be output
func SetLevel(level LogLevel) {
	defaultLevel = level
}

// SetShowPrefix controls whether log messages include level prefixes
func SetShowPrefix(show bool) {
	showPrefix = show
}

// Debug logs a debug message
func Debug(v ...interface{}) {
	if defaultLevel <= DebugLevel {
		if showPrefix {
			logWithPrefix("DEBUG: ", v...)
		} else {
			logWithPrefix("", v...)
		}
	}
}

// Debugf logs a formatted debug message
func Debugf(format string, v ...interface{}) {
	if defaultLevel <= DebugLevel {
		if showPrefix {
			logfWithPrefix("DEBUG: ", format, v...)
		} else {
			logfWithPrefix("", format, v...)
		}
	}
}

// Info logs an informational message
func Info(v ...interface{}) {
	if defaultLevel <= InfoLevel {
		logWithPrefix("", v...)
	}
}

// Infof logs a formatted informational message
func Infof(format string, v ...interface{}) {
	if defaultLevel <= InfoLevel {
		logfWithPrefix("", format, v...)
	}
}

// Warn logs a warning message
func Warn(v ...interface{}) {
	if defaultLevel <= WarnLevel {
		logWithPrefix("WARNING: ", v...)
	}
}

// Warnf logs a formatted warning message
func Warnf(format string, v ...interface{}) {
	if defaultLevel <= WarnLevel {
		logfWithPrefix("WARNING: ", format, v...)
	}
}

// Error logs an error message
func Error(v ...interface{}) {
	if defaultLevel <= ErrorLevel {
		logWithPrefix("ERROR: ", v...)
	}
}

// Errorf logs a formatted error message
func Errorf(format string, v ...interface{}) {
	if defaultLevel <= ErrorLevel {
		logfWithPrefix("ERROR: ", format, v...)
	}
}

// Fatal logs a critical error message and exits the application
func Fatal(v ...interface{}) {
	if defaultLevel <= FatalLevel {
		logWithPrefix("FATAL: ", v...)
		os.Exit(1)
	}
}

// Fatalf logs a formatted critical error message and exits the application
func Fatalf(format string, v ...interface{}) {
	if defaultLevel <= FatalLevel {
		logfWithPrefix("FATAL: ", format, v...)
		os.Exit(1)
	}
}

// logWithPrefix logs a message with an optional prefix
func logWithPrefix(prefix string, v ...interface{}) {
	if showPrefix {
		fmt.Fprint(defaultOutput, prefix)
	}
	fmt.Fprintln(defaultOutput, v...)
}

// logfWithPrefix logs a formatted message with an optional prefix
func logfWithPrefix(prefix string, format string, v ...interface{}) {
	if showPrefix {
		fmt.Fprint(defaultOutput, prefix)
	}
	fmt.Fprintf(defaultOutput, format+"\n", v...)
}

// CreateErrorf creates an error with a formatted message
// This is a utility function to replace fmt.Errorf
func CreateErrorf(format string, v ...interface{}) error {
	return fmt.Errorf(format, v...)
}

// ReadInput reads a line of input from stdin
// This is a utility function to replace fmt.Scanln
func ReadInput(result *string) error {
	_, err := fmt.Scanln(result)
	return err
}
