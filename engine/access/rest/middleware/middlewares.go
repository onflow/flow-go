package middleware

import (
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
)

func NotImplementedHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusNotImplemented)
	})
}

// LoggingMiddleware creates a middleware which adds a logger interceptor to each request to log the request method, uri,
// duration and response code
func LoggingMiddleware(logger zerolog.Logger) mux.MiddlewareFunc {
	return func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			start := time.Now()
			respWriter := newResponseWriter(w)
			handler.ServeHTTP(respWriter, req)
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
