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
	v1Subrouter := router.PathPrefix("/v1").Subrouter()

	lm := NewLoggingMiddleware(logger)
	// common middlewares for all request
	v1Subrouter.Use(lm.RequestStart())
	v1Subrouter.Use(ExpandableMiddleware())
	v1Subrouter.Use(SelectMiddleware())
	v1Subrouter.Use(lm.RequestEnd())

	for _, route := range apiRoutes(api) {
		v1Subrouter.
			Methods(route.Method).
			Path(route.Pattern).
			Name(route.Name).
			Handler(route.HandlerFunc)
	}

	return &http.Server{
		Addr:         listenAddress,
		Handler:      router,
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
	}
}

// apiRoutes returns the Gorilla Mux routes for each of the API defined in the rest definition
// currently, it only supports BlocksIdGet
func apiRoutes(api *APIHandler) generated.Routes {
	return generated.Routes{
		generated.Route{
			Name:        "AccountsAddressGet",
			Method:      strings.ToUpper("Get"),
			Pattern:     "/accounts/{address}",
			HandlerFunc: api.NotImplemented,
		},

		generated.Route{
			Name:        "BlocksGet",
			Method:      strings.ToUpper("Get"),
			Pattern:     "/blocks",
			HandlerFunc: api.NotImplemented,
		},

		generated.Route{
			Name:        "BlocksIdGet",
			Method:      strings.ToUpper("Get"),
			Pattern:     "/blocks/{id}",
			HandlerFunc: api.BlocksIdGet,
		},

		generated.Route{
			Name:        "CollectionsIdGet",
			Method:      strings.ToUpper("Get"),
			Pattern:     "/collections/{id}",
			HandlerFunc: api.NotImplemented,
		},

		generated.Route{
			Name:        "ExecutionResultsGet",
			Method:      strings.ToUpper("Get"),
			Pattern:     "/execution_results",
			HandlerFunc: api.NotImplemented,
		},

		generated.Route{
			Name:        "ExecutionResultsIdGet",
			Method:      strings.ToUpper("Get"),
			Pattern:     "/execution_results/{id}",
			HandlerFunc: api.NotImplemented,
		},

		generated.Route{
			Name:        "ScriptsPost",
			Method:      strings.ToUpper("Post"),
			Pattern:     "/scripts",
			HandlerFunc: api.NotImplemented,
		},

		generated.Route{
			Name:        "TransactionResultsTransactionIdGet",
			Method:      strings.ToUpper("Get"),
			Pattern:     "/transaction_results/{transaction_id}",
			HandlerFunc: api.NotImplemented,
		},

		generated.Route{
			Name:        "TransactionsIdGet",
			Method:      strings.ToUpper("Get"),
			Pattern:     "/transactions/{id}",
			HandlerFunc: api.NotImplemented,
		},

		generated.Route{
			Name:        "TransactionsPost",
			Method:      strings.ToUpper("Post"),
			Pattern:     "/transactions",
			HandlerFunc: api.NotImplemented,
		},
	}
}
