package middleware

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsMiddleware creates a middleware which adds a metrics interceptor to each request to track the
// number of requests,
func MetricsMiddleware(logger zerolog.Logger) mux.MiddlewareFunc {
	return func(inner http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			// // record star time
			// start := time.Now()
			// // modify the writer
			// respWriter := newResponseWriter(w)
			// // continue to the next handler
			// inner.ServeHTTP(respWriter, req)
			// log := logger.Info()
			// if respWriter.statusCode != http.StatusOK {
			// 	log = logger.Error()
			// }
			// log.Str("method", req.Method).
			// 	Str("uri", req.RequestURI).
			// 	Str("client_ip", req.RemoteAddr).
			// 	Str("user_agent", req.UserAgent()).
			// 	Dur("duration", time.Since(start)).
			// 	Int("response_code", respWriter.statusCode).
			// 	Msg("api")

			requestName := req.Method + " " + req.URL.String()
			reg := prometheus.NewRegistry()
			wrappedReg := prometheus.WrapRegistererWith(prometheus.Labels{"request": requestName}, reg)

			requestsTotal := promauto.With(wrappedReg).NewCounterVec(
				prometheus.CounterOpts{
					Name: "http_requests_total",
					Help: "Tracks the number of HTTP requests.",
				}, []string{"method", "code"},
			)
			requestDuration := promauto.With(wrappedReg).NewHistogramVec(
				prometheus.HistogramOpts{
					Name: "http_request_duration_seconds",
					Help: "Tracks the latencies for HTTP requests.",
				},
				[]string{"method", "code"},
			)
			requestSize := promauto.With(wrappedReg).NewSummaryVec(
				prometheus.SummaryOpts{
					Name: "http_request_size_bytes",
					Help: "Tracks the size of HTTP requests.",
				},
				[]string{"method", "code"},
			)
			responseSize := promauto.With(wrappedReg).NewSummaryVec(
				prometheus.SummaryOpts{
					Name: "http_response_size_bytes",
					Help: "Tracks the size of HTTP responses.",
				},
				[]string{"method", "code"},
			)

			promhttp.InstrumentHandlerRequestSize(
				requestSize,
				promhttp.InstrumentHandlerCounter(
					requestsTotal,
					promhttp.InstrumentHandlerResponseSize(
						responseSize,
						promhttp.InstrumentHandlerDuration(
							requestDuration,
							http.HandlerFunc(func(writer http.ResponseWriter, r *http.Request) {
								logger.Info().Msg("Metrics for " + requestName + " stored successfully")
								inner.ServeHTTP(writer, r)
							}),
						),
					),
				),
			)

			logger.Info().Msg("Returning HTTP at end of middleware")
			// modify the writer
			respWriter := newResponseWriter(w)
			// continue to the next handler
			inner.ServeHTTP(respWriter, req)
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
