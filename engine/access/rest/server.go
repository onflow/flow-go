package rest

import (
	"net/http"
	"strings"
	"time"

	"github.com/rs/cors"

	"github.com/onflow/flow-go/engine/access/rest/middleware"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/access/rest/generated"
)

// NewServer returns an HTTP server initialized with the REST API handler
func NewServer(handlers *Handlers, listenAddress string, logger zerolog.Logger) *http.Server {

	router := mux.NewRouter().StrictSlash(true)
	v1SubRouter := router.PathPrefix("/v1").Subrouter()

	// common middleware for all request
	v1SubRouter.Use(middleware.LoggingMiddleware(logger))
	v1SubRouter.Use(middleware.QueryExpandable())
	v1SubRouter.Use(middleware.QuerySelect())

	for _, route := range apiRoutes(handlers) {
		v1SubRouter.
			Methods(route.Method).
			Path(route.Pattern).
			Name(route.Name).
			Handler(route.HandlerFunc)
	}

	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedHeaders: []string{"*"},
		AllowedMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodDelete,
			http.MethodOptions,
			http.MethodHead},
	})

	return &http.Server{
		Addr:         listenAddress,
		Handler:      c.Handler(router),
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
	}
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
