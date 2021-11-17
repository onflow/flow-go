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

	for _, h := range apiHandlers(logger, backend) {
		v1SubRouter.
			Methods(h.method).
			Path(h.pattern).
			Name(h.name).
			Handler(h)
	}

	return &http.Server{
		Addr:         listenAddress,
		Handler:      router,
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
	}
}

func apiHandlers(logger zerolog.Logger, backend access.API) []*Handler {
	return []*Handler{
		// Transactions
		{
			logger:      logger,
			backend:     backend,
			method:      "GET",
			pattern:     "/transactions/{id}",
			name:        "getTransactionByID",
			handlerFunc: getTransactionByID,
		}, {
			logger:      logger,
			backend:     backend,
			method:      "POST",
			pattern:     "/transactions",
			name:        "createTransaction",
			handlerFunc: createTransaction,
		},
		// Blocks
		{
			logger:      logger,
			backend:     backend,
			method:      "GET",
			pattern:     "/blocks/{id}",
			name:        "getBlocksByID",
			handlerFunc: getBlocksByID,
		}, {
			logger:      logger,
			backend:     backend,
			method:      "GET",
			pattern:     "/blocks",
			handlerFunc: NotImplemented,
		},
		// Collections
		{
			logger:      logger,
			backend:     backend,
			method:      "GET",
			pattern:     "/collections/{id}",
			handlerFunc: NotImplemented,
		}, {}}
}
