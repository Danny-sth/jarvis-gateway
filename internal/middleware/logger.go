package middleware

import (
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// Paths excluded from access logging (noisy endpoints)
var excludedPaths = []string{
	"/api/system/metrics",
	"/api/monitoring/",
	"/health",
}

func shouldSkipLogging(path string) bool {
	for _, excluded := range excludedPaths {
		if strings.HasPrefix(path, excluded) {
			return true
		}
	}
	return false
}

type responseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
	size        int
}

func (rw *responseWriter) WriteHeader(code int) {
	if rw.wroteHeader {
		return
	}
	rw.status = code
	rw.wroteHeader = true
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.size += n
	return n, err
}

// Logger returns a middleware that logs HTTP requests using structured logging (slog).
// Excludes noisy endpoints like /health and /api/system/metrics from logging.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rw, r)

		// Skip logging for noisy endpoints
		if shouldSkipLogging(r.URL.Path) {
			return
		}

		duration := time.Since(start)

		// Log with structured fields
		slog.Info("http request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", rw.status),
			slog.Duration("duration", duration),
			slog.Int("size", rw.size),
			slog.String("remote_addr", r.RemoteAddr),
			slog.String("user_agent", r.UserAgent()),
		)
	})
}

// LoggerWithLevel returns a logger middleware that uses different log levels
// based on response status code. Excludes noisy endpoints from INFO logging
// but still logs errors and warnings for all paths.
func LoggerWithLevel(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rw, r)

		duration := time.Since(start)

		attrs := []any{
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", rw.status),
			slog.Duration("duration", duration),
			slog.Int("size", rw.size),
			slog.String("remote_addr", r.RemoteAddr),
		}

		// Choose log level based on status
		// Always log errors/warnings, but skip INFO for noisy endpoints
		switch {
		case rw.status >= 500:
			slog.Error("http request", attrs...)
		case rw.status >= 400:
			slog.Warn("http request", attrs...)
		default:
			if !shouldSkipLogging(r.URL.Path) {
				slog.Info("http request", attrs...)
			}
		}
	})
}
