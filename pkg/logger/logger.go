// Package logger provides unified logging functionality for the nigiri CLI application
package logger

import (
	"fmt"
	"io"
	"os"
)

// LogLevel represents the severity of a log message.
type LogLevel int

const (
	// DebugLevel logs verbose development info.
	DebugLevel LogLevel = iota
	// InfoLevel logs general information.
	InfoLevel
	// WarnLevel logs warnings that might cause issues.
	WarnLevel
	// ErrorLevel logs errors that prevented an operation.
	ErrorLevel
	// FatalLevel logs critical errors and exits the application.
	FatalLevel
)

var (
	// Default output is stderr.
	defaultOutput io.Writer = os.Stderr
	// Default log level is Info.
	defaultLevel = InfoLevel
	// Whether to include log level prefix in output.
	showPrefix = true
)

// SetOutput changes the output destination for the logger.
func SetOutput(w io.Writer) {
	defaultOutput = w
}

// SetLevel changes the minimum log level that will be output.
func SetLevel(level LogLevel) {
	defaultLevel = level
}

// SetShowPrefix controls whether log messages include level prefixes.
func SetShowPrefix(show bool) {
	showPrefix = show
}

// Debug logs a debug message.
func Debug(v ...any) {
	if defaultLevel <= DebugLevel {
		if showPrefix {
			logWithPrefix("DEBUG: ", v...)
		} else {
			logWithPrefix("", v...)
		}
	}
}

// Debugf logs a formatted debug message.
func Debugf(format string, v ...any) {
	if defaultLevel <= DebugLevel {
		if showPrefix {
			logWithPrefixf("DEBUG: ", format, v...)
		} else {
			logWithPrefixf("", format, v...)
		}
	}
}

// Info logs an informational message.
func Info(v ...any) {
	if defaultLevel <= InfoLevel {
		logWithPrefix("", v...)
	}
}

// Infof logs a formatted informational message.
func Infof(format string, v ...any) {
	if defaultLevel <= InfoLevel {
		logWithPrefixf("", format, v...)
	}
}

// Warn logs a warning message.
func Warn(v ...any) {
	if defaultLevel <= WarnLevel {
		logWithPrefix("WARNING: ", v...)
	}
}

// Warnf logs a formatted warning message.
func Warnf(format string, v ...any) {
	if defaultLevel <= WarnLevel {
		logWithPrefixf("WARNING: ", format, v...)
	}
}

// Error logs an error message.
func Error(v ...any) {
	if defaultLevel <= ErrorLevel {
		logWithPrefix("ERROR: ", v...)
	}
}

// Errorf logs a formatted error message.
func Errorf(format string, v ...any) {
	if defaultLevel <= ErrorLevel {
		logWithPrefixf("ERROR: ", format, v...)
	}
}

// Fatal logs a critical error message and exits the application.
func Fatal(v ...any) {
	if defaultLevel <= FatalLevel {
		logWithPrefix("FATAL: ", v...)
		os.Exit(1)
	}
}

// Fatalf logs a formatted critical error message and exits the application.
func Fatalf(format string, v ...any) {
	if defaultLevel <= FatalLevel {
		logWithPrefixf("FATAL: ", format, v...)
		os.Exit(1)
	}
}

// logWithPrefix logs a message with an optional prefix.
func logWithPrefix(prefix string, v ...any) {
	if showPrefix {
		fmt.Fprint(defaultOutput, prefix)
	}

	fmt.Fprintln(defaultOutput, v...)
}

// logWithPrefixf logs a formatted message with an optional prefix.
func logWithPrefixf(prefix string, format string, v ...any) {
	if showPrefix {
		fmt.Fprint(defaultOutput, prefix)
	}

	fmt.Fprintf(defaultOutput, format+"\n", v...)
}

// CreateErrorf formats an error like [fmt.Errorf], including %w wrapping.
func CreateErrorf(format string, v ...any) error {
	return fmt.Errorf(format, v...)
}

// ReadInput reads one line of space-separated input from stdin into result,
// like [fmt.Scanln].
func ReadInput(result *string) error {
	_, err := fmt.Scanln(result)

	return err //nolint:wrapcheck // callers report it as a failed confirmation
}
