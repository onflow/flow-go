package middleware

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
)

// LoggingMiddleware creates a middleware which adds a logger interceptor to each request to log the request method, uri,
// duration and response code. Both start and finish logs are at DEBUG level.
//
// Example log messages for searching:
//   - Start:  DBG "started REST request" method=GET uri=/v1/blocks client_ip=... user_agent=...
//   - Finish: DBG "finished REST request" method=GET uri=/v1/blocks client_ip=... user_agent=... duration=... response_code=200
func LoggingMiddleware(logger zerolog.Logger) mux.MiddlewareFunc {
	return func(inner http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			lg := logger.With().
				Str("method", req.Method).
				Str("uri", req.RequestURI).
				Str("client_ip", req.RemoteAddr).
				Str("user_agent", req.UserAgent()).
				Logger()

			// Log at start of request for debugging and tracing
			lg.Debug().Msg("started REST request")

			// record start time
			start := time.Now()
			// modify the writer
			respWriter := newResponseWriter(w)
			// continue to the next handler
			inner.ServeHTTP(respWriter, req)

			// Log at end of request with response details
			lg.Debug().
				Dur("duration", time.Since(start)).
				Int("response_code", respWriter.statusCode).
				Msg("finished REST request")
		})
	}
}

// responseWriter is a wrapper around http.ResponseWriter and helps capture the response code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

// http.Hijacker necessary for using middleware with gorilla websocket connections.
var _ http.Hijacker = (*responseWriter)(nil)

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{w, http.StatusOK}
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := rw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("hijacking not supported")
	}
	return hijacker.Hijack()
}
