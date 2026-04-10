// Package logger sets up a dual-output logger that writes to both stdout and
// cocobase.log in the working directory. All standard log.Print* calls are
// redirected here automatically after Init().
package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const logFile = "cocobase.log"

var (
	infoLogger  *log.Logger
	errorLogger *log.Logger
	file        *os.File
)

// Init opens (or creates) the log file and redirects the standard logger.
func Init() error {
	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file %s: %w", logFile, err)
	}
	file = f

	multi := io.MultiWriter(os.Stdout, f)
	errMulti := io.MultiWriter(os.Stderr, f)

	infoLogger = log.New(multi, "", 0)
	errorLogger = log.New(errMulti, "", 0)

	// Redirect standard logger
	log.SetOutput(multi)
	log.SetFlags(0)

	return nil
}

// Close flushes and closes the log file.
func Close() {
	if file != nil {
		file.Close()
	}
}

// LogFile returns the absolute path to the log file.
func LogFile() string {
	abs, _ := filepath.Abs(logFile)
	return abs
}

func ts() string { return time.Now().Format("2006/01/02 15:04:05") }

func callerStr(skip int) string {
	_, f, line, ok := runtime.Caller(skip + 1)
	if !ok {
		return "unknown"
	}
	parts := strings.Split(f, "/")
	if len(parts) > 2 {
		parts = parts[len(parts)-2:]
	}
	return fmt.Sprintf("%s:%d", strings.Join(parts, "/"), line)
}

// Info logs an informational message to stdout + file.
func Info(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	infoLogger.Printf("%s %s", ts(), msg)
}

// Error logs an error with source file:line — goes to stderr + file.
func Error(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	errorLogger.Printf("%s [ERROR] (%s) %s", ts(), callerStr(1), msg)
}

// Errorf is like Error but accepts a pre-formatted string (no skip adjustment needed).
func Errorf(skip int, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	errorLogger.Printf("%s [ERROR] (%s) %s", ts(), callerStr(skip+1), msg)
}

// Fatal logs an error and exits.
func Fatal(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	errorLogger.Printf("%s [FATAL] (%s) %s", ts(), callerStr(1), msg)
	os.Exit(1)
}

// RequestLog holds everything captured about a single HTTP request/response.
type RequestLog struct {
	// Request
	Method      string
	Path        string
	Query       string
	IP          string
	UserAgent   string
	APIKey      string // masked
	AuthHeader  string // masked
	ContentType string
	BodySize    int
	// Response
	Status  int
	Latency time.Duration
	// Error (optional)
	Err string
}

// LogRequest writes a structured request log line. Errors (5xx) go to the
// error logger with extra detail; others go to info.
func LogRequest(r RequestLog) {
	query := ""
	if r.Query != "" {
		query = "?" + r.Query
	}

	// Mask sensitive headers — show only first 8 chars
	apiKey := mask(r.APIKey)
	auth := mask(r.AuthHeader)

	status := r.Status
	latency := r.Latency.Round(time.Millisecond)

	if status >= 500 {
		errorLogger.Printf(
			"%s [ERROR] %d %s %s%s\n"+
				"         ip=%-15s ua=%s\n"+
				"         api-key=%s auth=%s content-type=%s body=%dB\n"+
				"         latency=%v error=%s",
			ts(), status, r.Method, r.Path, query,
			r.IP, r.UserAgent,
			apiKey, auth, r.ContentType, r.BodySize,
			latency, r.Err,
		)
	} else if status >= 400 {
		infoLogger.Printf(
			"%s [WARN]  %d %s %s%s ip=%s latency=%v",
			ts(), status, r.Method, r.Path+query, query, r.IP, latency,
		)
	} else {
		infoLogger.Printf(
			"%s [INFO]  %d %s %s%s latency=%v",
			ts(), status, r.Method, r.Path, query, latency,
		)
	}
}

func mask(s string) string {
	if s == "" {
		return "-"
	}
	if len(s) <= 8 {
		return strings.Repeat("•", len(s))
	}
	return s[:8] + "••••"
}
