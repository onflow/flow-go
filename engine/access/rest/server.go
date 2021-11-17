package rest

import (
	"net/http"
	"strings"
	"time"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/generated"
	"github.com/onflow/flow-go/engine/access/rest/middleware"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
)

// NewServer returns an HTTP server initialized with the REST API handler
func NewServer(handlers *Handlers, backend access.API, listenAddress string, logger zerolog.Logger) *http.Server {

	router := mux.NewRouter().StrictSlash(true)
	v1SubRouter := router.PathPrefix("/v1").Subrouter()

	lm := middleware.NewLoggingMiddleware(logger)
	// common middleware for all request
	v1SubRouter.Use(lm.RequestStart())
	v1SubRouter.Use(middleware.QueryExpandable())
	v1SubRouter.Use(middleware.QuerySelect())
	v1SubRouter.Use(lm.RequestEnd())

	for _, route := range apiRoutes(handlers) {
		v1SubRouter.
			Methods(route.Method).
			Path(route.Pattern).
			Name(route.Name).
			Handler(route.HandlerFunc)
	}

	for _, h := range apiHandlers(logger, backend) {
		v1SubRouter.
			Methods(h.method).
			Path(h.pattern).
			Name(h.name).
			HandlerFunc(h.ServeHTTP)
	}

	return &http.Server{
		Addr:         listenAddress,
		Handler:      router,
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
	}
}

func apiHandlers(logger zerolog.Logger, backend access.API) []Handler {
	return []Handler{{
		logger:      logger,
		backend:     backend,
		method:      "GET",
		pattern:     "/transactions/{id}",
		name:        "getTransactionByID",
		handlerFunc: getTransactionByID,
	}}
}

// apiRoutes returns the Gorilla Mux routes for each of the API defined in the rest definition
// currently, it only supports BlocksIdGet
func apiRoutes(handlers *Handlers) generated.Routes {
	return generated.Routes{
		generated.Route{
			Name:        "AccountsAddressGet",
			Method:      strings.ToUpper("Get"),
			Pattern:     "/accounts/{address}",
			HandlerFunc: handlers.NotImplemented,
		},

		generated.Route{
			Name:        "BlocksGet",
			Method:      strings.ToUpper("Get"),
			Pattern:     "/blocks",
			HandlerFunc: handlers.NotImplemented,
		},

		generated.Route{
			Name:        "BlocksIdGet",
			Method:      strings.ToUpper("Get"),
			Pattern:     "/blocks/{id}",
			HandlerFunc: handlers.BlocksIdGet,
		},

		generated.Route{
			Name:        "CollectionsIdGet",
			Method:      strings.ToUpper("Get"),
			Pattern:     "/collections/{id}",
			HandlerFunc: handlers.NotImplemented,
		},

		generated.Route{
			Name:        "ExecutionResultsGet",
			Method:      strings.ToUpper("Get"),
			Pattern:     "/execution_results",
			HandlerFunc: handlers.NotImplemented,
		},

		generated.Route{
			Name:        "ExecutionResultsIdGet",
			Method:      strings.ToUpper("Get"),
			Pattern:     "/execution_results/{id}",
			HandlerFunc: handlers.NotImplemented,
		},

		generated.Route{
			Name:        "ScriptsPost",
			Method:      strings.ToUpper("Post"),
			Pattern:     "/scripts",
			HandlerFunc: handlers.NotImplemented,
		},

		generated.Route{
			Name:        "TransactionResultsTransactionIdGet",
			Method:      strings.ToUpper("Get"),
			Pattern:     "/transaction_results/{transaction_id}",
			HandlerFunc: handlers.NotImplemented,
		},

		generated.Route{
			Name:        "TransactionsIdGet",
			Method:      strings.ToUpper("Get"),
			Pattern:     "/transactions/{id}",
			HandlerFunc: handlers.NotImplemented,
		},

		generated.Route{
			Name:        "TransactionsPost",
			Method:      strings.ToUpper("Post"),
			Pattern:     "/transactions",
			HandlerFunc: handlers.NotImplemented,
		},
	}
}
