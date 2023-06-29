package middleware

import (
	"net/http"

	"github.com/slok/go-http-metrics/middleware"
	"github.com/slok/go-http-metrics/middleware/std"

	"github.com/gorilla/mux"

	"github.com/onflow/flow-go/module"
)

func MetricsMiddleware(restCollector module.RestMetrics, urlToRoute func(string) (string, error)) mux.MiddlewareFunc {
	metricsMiddleware := middleware.New(middleware.Config{Recorder: restCollector})
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			//urlToRoute transforms specific URL to generic url pattern
			routeName, err := urlToRoute(req.URL.Path)
			if err != nil {
				// In case of an error, an empty route name filled with "unknown"
				routeName = "unknown"
			}

			// This is a custom metric being called on every http request
			restCollector.AddTotalRequests(req.Context(), req.Method, routeName)

			// Modify the writer
			respWriter := &responseWriter{w, http.StatusOK}

			// Record go-http-metrics/middleware metrics and continue to the next handler
			std.Handler("", metricsMiddleware, next).ServeHTTP(respWriter, req)
		})
	}
}
