package rest

import (
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/access/rest/generated"
)

// NewRestAPIServer returns an HTTP server initialized with the REST API handler
func NewRestAPIServer(api *APIHandler, listenAddress string, logger zerolog.Logger) *http.Server {

	router := mux.NewRouter().StrictSlash(true)
	for _, route := range apiRoutes(api) {
		handler := newHandler(route.HandlerFunc, route.Name, logger)
		router.
			Methods(route.Method).
			Path(route.Pattern).
			Name(route.Name).
			Handler(handler)
	}

	return &http.Server{
		Addr:    listenAddress,
		Handler: router,
	}
}

// apiRoutes returns the Gorilla Mux routes for each of the API defined in the rest definition
// currently, it only supports BlocksIdGet
func apiRoutes(api *APIHandler) generated.Routes {
	return generated.Routes{
		generated.Route{
			Name:        "AccountsAddressGet",
			Method:      strings.ToUpper("Get"),
			Pattern:     "/v1/accounts/{address}",
			HandlerFunc: api.NotImplemented,
		},

		generated.Route{
			Name:        "BlocksGet",
			Method:      strings.ToUpper("Get"),
			Pattern:     "/v1/blocks",
			HandlerFunc: api.NotImplemented,
		},

		generated.Route{
			Name:        "BlocksIdGet",
			Method:      strings.ToUpper("Get"),
			Pattern:     "/v1/blocks/{id}",
			HandlerFunc: api.BlocksIdGet,
		},

		generated.Route{
			Name:        "CollectionsIdGet",
			Method:      strings.ToUpper("Get"),
			Pattern:     "/v1/collections/{id}",
			HandlerFunc: api.NotImplemented,
		},

		generated.Route{
			Name:        "ExecutionResultsGet",
			Method:      strings.ToUpper("Get"),
			Pattern:     "/v1/execution_results",
			HandlerFunc: api.NotImplemented,
		},

		generated.Route{
			Name:        "ExecutionResultsIdGet",
			Method:      strings.ToUpper("Get"),
			Pattern:     "/v1/execution_results/{id}",
			HandlerFunc: api.NotImplemented,
		},

		generated.Route{
			Name:        "ScriptsPost",
			Method:      strings.ToUpper("Post"),
			Pattern:     "/v1/scripts",
			HandlerFunc: api.NotImplemented,
		},

		generated.Route{
			Name:        "TransactionResultsTransactionIdGet",
			Method:      strings.ToUpper("Get"),
			Pattern:     "/v1/transaction_results/{transaction_id}",
			HandlerFunc: api.NotImplemented,
		},

		generated.Route{
			Name:        "TransactionsIdGet",
			Method:      strings.ToUpper("Get"),
			Pattern:     "/v1/transactions/{id}",
			HandlerFunc: api.NotImplemented,
		},

		generated.Route{
			Name:        "TransactionsPost",
			Method:      strings.ToUpper("Post"),
			Pattern:     "/v1/transactions",
			HandlerFunc: api.NotImplemented,
		},
	}
}

// newHandler creates a request handler which adds a logger interceptor to each request to log the request method, uri,
// duration and response code
func newHandler(inner http.Handler, name string, logger zerolog.Logger) http.Handler {
	apiLogger := logger.With().Str("route_name", name).Logger()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		respWriter := newResponseWriter(w)
		inner.ServeHTTP(respWriter, r)
		if respWriter.statusCode == http.StatusOK {
			apiLogger.Info().Str("method", r.Method).
				Str("uri", r.RequestURI).
				Str("client_ip", r.RemoteAddr).
				Str("user_agent", r.UserAgent()).
				Dur("duration", time.Since(start)).
				Int("response_code", respWriter.statusCode).
				Msg("api")
		} else {
			apiLogger.Error().Str("method", r.Method).
				Str("uri", r.RequestURI).
				Str("client_ip", r.RemoteAddr).
				Str("user_agent", r.UserAgent()).
				Dur("duration", time.Since(start)).
				Int("response_code", respWriter.statusCode).
				Msg("api")
		}
	})
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
