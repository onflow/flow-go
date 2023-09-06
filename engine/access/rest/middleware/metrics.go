package middleware

import (
	"net/http"

	"github.com/slok/go-http-metrics/middleware"
	"github.com/slok/go-http-metrics/middleware/std"

	"github.com/gorilla/mux"

	"github.com/onflow/flow-go/module"
)

func MetricsMiddleware(restCollector module.RestMetrics) mux.MiddlewareFunc {
	metricsMiddleware := middleware.New(middleware.Config{Recorder: restCollector})
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			// This is a custom metric being called on every http request
			restCollector.AddTotalRequests(req.Context(), req.Method, req.URL.Path)

			// Modify the writer
			respWriter := &responseWriter{w, http.StatusOK}

			// Record go-http-metrics/middleware metrics and continue to the next handler
			std.Handler("", metricsMiddleware, next).ServeHTTP(respWriter, req)
		})
	}
}
