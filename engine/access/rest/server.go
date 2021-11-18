package rest

import (
	"net/http"
	"time"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/middleware"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
)

// NewServer returns an HTTP server initialized with the REST API handler
func NewServer(backend access.API, listenAddress string, logger zerolog.Logger) *http.Server {

	router := mux.NewRouter().StrictSlash(true)
	v1SubRouter := router.PathPrefix("/v1").Subrouter()

	lm := middleware.NewLoggingMiddleware(logger)
	// common middleware for all request
	v1SubRouter.Use(lm.RequestStart())
	v1SubRouter.Use(middleware.QueryExpandable())
	v1SubRouter.Use(middleware.QuerySelect())
	v1SubRouter.Use(lm.RequestEnd())

	for _, r := range routeDefinitions() {
		h := NewHandler(logger, backend, r.apiHandlerFunc)
		route := v1SubRouter.
			Methods(h.method).
			Path(h.pattern).
			Name(h.name).
			Handler(h)
		h.route = route
	}

	return &http.Server{
		Addr:         listenAddress,
		Handler:      router,
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
	}
}

type routeDefinition struct {
	name        string
	method      string
	pattern     string
	apiHandlerFunc ApiHandlerFunc
}

func routeDefinitions() []routeDefinition {
	return []routeDefinition{
		// Transactions
		{
			method:      "GET",
			pattern:     "/transactions/{id}",
			name:        "getTransactionByID",
			apiHandlerFunc: getTransactionByID,
		}, {
			method:      "POST",
			pattern:     "/transactions",
			name:        "createTransaction",
			apiHandlerFunc: createTransaction,
		},
		// Blocks
		{
			method:      "GET",
			pattern:     "/blocks/{id}",
			name:        "getBlocksByID",
			apiHandlerFunc: getBlocksByID,
		}, {
			method:      "GET",
			pattern:     "/blocks",
			apiHandlerFunc: NotImplemented,
		},
		// Collections
		{
			method:      "GET",
			pattern:     "/collections/{id}",
			apiHandlerFunc: NotImplemented,
		}, {}}
}

