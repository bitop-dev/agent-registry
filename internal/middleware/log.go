// Package middleware provides HTTP middleware for the registry server.
// Logging follows the wide-event / canonical log line pattern:
// one structured JSON event is emitted per request, containing every piece
// of context accumulated across the request lifecycle.
package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"sync"
	"time"

	"github.com/bitop-dev/agent-registry/internal/metrics"
)

type ctxKey struct{}

// RequestEvent accumulates context fields throughout a request lifecycle.
// Handlers enrich it via AddField; the middleware emits it once at the end.
type RequestEvent struct {
	mu     sync.Mutex
	fields []any
}

func (e *RequestEvent) add(args ...any) {
	e.mu.Lock()
	e.fields = append(e.fields, args...)
	e.mu.Unlock()
}

// AddField attaches a key-value pair to the current request's wide event.
// Call this from any handler to enrich the log with business context
// (e.g. package name, version, runtime type, error detail).
func AddField(ctx context.Context, key string, val any) {
	if e, ok := ctx.Value(ctxKey{}).(*RequestEvent); ok {
		e.add(key, val)
	}
}

// responseWriter wraps http.ResponseWriter to capture the status code and
// number of bytes written so the middleware can include them in the wide event.
type responseWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (rw *responseWriter) WriteHeader(status int) {
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.bytes += n
	return n, err
}

// Logging returns middleware that emits one wide event per request.
// The event includes: request_id, method, path, status, duration_ms, bytes,
// remote_addr, user_agent, and any fields added by handlers via AddField.
//
// Log level is chosen by outcome:
//   - 5xx → Error
//   - 4xx → Warn
//   - 2xx/3xx → Info
//
// The request_id is also written to the X-Request-ID response header so
// callers can correlate client-side errors with server logs.
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		reqID := fmt.Sprintf("req_%016x", rand.Uint64())

		event := &RequestEvent{}
		ctx := context.WithValue(r.Context(), ctxKey{}, event)
		r = r.WithContext(ctx)

		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		w.Header().Set("X-Request-ID", reqID)

		next.ServeHTTP(rw, r)

		duration := time.Since(start)

		level := slog.LevelInfo
		if rw.status >= 500 {
			level = slog.LevelError
		} else if rw.status >= 400 {
			level = slog.LevelWarn
		}

		fields := []any{
			"request_id", reqID,
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.status,
			"duration_ms", duration.Milliseconds(),
			"bytes", rw.bytes,
			"remote_addr", r.RemoteAddr,
		}
		if ua := r.Header.Get("User-Agent"); ua != "" {
			fields = append(fields, "user_agent", ua)
		}

		// Append all business context accumulated by handlers.
		event.mu.Lock()
		fields = append(fields, event.fields...)
		event.mu.Unlock()

		slog.Log(ctx, level, "request", fields...)
		metrics.Record(rw.status, duration.Milliseconds())
	})
}
