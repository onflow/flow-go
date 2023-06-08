package middleware

import (
	"net/http"

	"github.com/slok/go-http-metrics/middleware"
	"github.com/slok/go-http-metrics/middleware/std"

	"github.com/gorilla/mux"

	"github.com/onflow/flow-go/module"
)

func MetricsMiddleware(restCollector module.RestMetrics) mux.MiddlewareFunc {
	cfg := middleware.Config{Recorder: restCollector}
	serviceID := cfg.Service
	metricsMiddleware := middleware.New(cfg)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			// This is a custom metric being called on every http request
			restCollector.AddTotalRequests(req.Context(), serviceID, req.Method)

			// Modify the writer
			respWriter := &responseWriter{w, http.StatusOK}

			// Record go-http-metrics/middleware metrics and continue to the next handler
			std.Handler("", metricsMiddleware, next).ServeHTTP(respWriter, req)
		})
	}
}
