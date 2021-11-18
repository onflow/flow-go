package middleware

import (
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
)

// LoggingMiddleware creates a middleware which adds a logger interceptor to each request to log the request method, uri,
// duration and response code
func LoggingMiddleware(logger zerolog.Logger) mux.MiddlewareFunc {
	return func(inner http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			// record star time
			start := time.Now()
			// modify the writer
			respWriter := newResponseWriter(w)
			// continue to the next handler
			inner.ServeHTTP(respWriter, req)
			if respWriter.statusCode == http.StatusOK {
				logger.Info().Str("method", req.Method).
					Str("uri", req.RequestURI).
					Str("client_ip", req.RemoteAddr).
					Str("user_agent", req.UserAgent()).
					Dur("duration", time.Since(start)).
					Int("response_code", respWriter.statusCode).
					Msg("api")
			} else {
				logger.Error().Str("method", req.Method).
					Str("uri", req.RequestURI).
					Str("client_ip", req.RemoteAddr).
					Str("user_agent", req.UserAgent()).
					Dur("duration", time.Since(start)).
					Int("response_code", respWriter.statusCode).
					Msg("api")
			}
		})
	}
}

// responseWriter is a wrapper around http.ResponseWriter and helps capture the response code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{w, http.StatusOK}
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
