package rest

import (
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
)

// LoggingMiddleware is a middleware which adds a logger interceptor to each request to log the request method, uri,
// duration and response code.
// To use the middleware add the middleware returned by RequestStart() before all the other middlewares and the one
// returned by RequestEnd() as the last middleware
type LoggingMiddleware struct {
	logger           zerolog.Logger
	requestStartTime time.Time
}

func NewLoggingMiddleware(logger zerolog.Logger) *LoggingMiddleware {
	return &LoggingMiddleware{
		logger: logger,
	}
}

func (lm *LoggingMiddleware) RequestStart() mux.MiddlewareFunc {
	return func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			lm.requestStartTime = time.Now()
			respWriter := newResponseWriter(w)
			handler.ServeHTTP(respWriter, req)
		})
	}
}

func (lm *LoggingMiddleware) RequestEnd() mux.MiddlewareFunc {

	return func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, req *http.Request) {

			if lm.requestStartTime.IsZero() {
				lm.logger.Warn().Msg("LoggingMiddleWare start time not set. Please add LoggingMiddlewareStart middleware to the route")
			}

			respWriter, ok := writer.(*responseWriter)
			if !ok {
				lm.logger.Warn().Msg("incorrect response writer")
				handler.ServeHTTP(writer, req)
				return
			}

			if respWriter.statusCode == http.StatusOK {
				lm.logger.Info().Str("method", req.Method).
					Str("uri", req.RequestURI).
					Str("client_ip", req.RemoteAddr).
					Str("user_agent", req.UserAgent()).
					Dur("duration", time.Since(lm.requestStartTime)).
					Int("response_code", respWriter.statusCode).
					Msg("api")
			} else {
				lm.logger.Error().Str("method", req.Method).
					Str("uri", req.RequestURI).
					Str("client_ip", req.RemoteAddr).
					Str("user_agent", req.UserAgent()).
					Dur("duration", time.Since(lm.requestStartTime)).
					Int("response_code", respWriter.statusCode).
					Msg("api")
			}

			// continue request handling with original response writer
			handler.ServeHTTP(respWriter.ResponseWriter, req)
		})
	}
}

// responseWriter is a wrapper around http.ResponseWriter and helps capture the response code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

var _ http.ResponseWriter = &responseWriter{}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{w, http.StatusOK}
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
