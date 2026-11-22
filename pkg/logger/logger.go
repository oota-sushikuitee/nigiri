// Package logger writes prefixed warning and error lines for the nigiri CLI.
package logger

import (
	"fmt"
	"io"
	"os"
)

var output io.Writer = os.Stderr

// SetOutput redirects log output, which defaults to os.Stderr.
func SetOutput(w io.Writer) {
	output = w
}

// Warnf logs a formatted warning message prefixed with "WARNING: ".
func Warnf(format string, v ...any) {
	fmt.Fprintf(output, "WARNING: "+format+"\n", v...)
}

// Error logs an error message prefixed with "ERROR: ".
func Error(v ...any) {
	fmt.Fprint(output, "ERROR: ")
	fmt.Fprintln(output, v...)
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
