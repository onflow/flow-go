package middleware

import (
	"net/http"

	"github.com/onflow/flow-go/module/metrics"

	"github.com/slok/go-http-metrics/middleware"
	"github.com/slok/go-http-metrics/middleware/std"

	metricsProm "github.com/slok/go-http-metrics/metrics/prometheus"

	"github.com/gorilla/mux"
)

func MetricsMiddleware() mux.MiddlewareFunc {
	r := metrics.NewRestCollector(metricsProm.Config{Prefix: "access_rest_api"})
	metricsMiddleware := middleware.New(middleware.Config{Recorder: r})

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			// This is a custom metric being called on every http request
			r.AddTotalRequests(req.Context(), req.Method, req.URL.Path)

			// Modify the writer
			respWriter := &responseWriter{w, http.StatusOK}

			// Record go-http-metrics/middleware metrics and continue to the next handler
			std.Handler("", metricsMiddleware, next).ServeHTTP(respWriter, req)
		})
	}
}
