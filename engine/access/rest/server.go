package rest

import (
	"net/http"
	"time"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/middleware"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
)

// all route names
const (
	getTransactionByIDRoute = "getTransactionByID"
	createTransactionRoute  = "createTransaction"
	getBlocksByIDRoute      = "getBlocksByID"
	getBlocksByHeightRoute  = "getBlocksByHeightRoute"
	getCollectionByIDRoute  = "getCollectionByIDRoute"
)

// NewServer returns an HTTP server initialized with the REST API handler
func NewServer(backend access.API, listenAddress string, logger zerolog.Logger) *http.Server {

	router := initRouter(backend, logger)

	return &http.Server{
		Addr:         listenAddress,
		Handler:      router,
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
	}
}

func initRouter(backend access.API, logger zerolog.Logger) *mux.Router {
	router := mux.NewRouter().StrictSlash(true)
	v1SubRouter := router.PathPrefix("/v1").Subrouter()

	lm := middleware.NewLoggingMiddleware(logger)
	// common middleware for all request
	v1SubRouter.Use(lm.RequestStart())
	v1SubRouter.Use(middleware.QueryExpandable())
	v1SubRouter.Use(middleware.QuerySelect())
	v1SubRouter.Use(lm.RequestEnd())

	var linkGenerator LinkGenerator = NewLinkGeneratorImpl(v1SubRouter)

	for _, r := range routeDefinitions() {
		h := NewHandler(logger, backend, r.apiHandlerFunc, linkGenerator)
		v1SubRouter.
			Methods(r.method).
			Path(r.pattern).
			Name(r.name).
			Handler(h)
	}
	return router
}

type routeDefinition struct {
	name           string
	method         string
	pattern        string
	apiHandlerFunc ApiHandlerFunc
}

func routeDefinitions() []routeDefinition {
	return []routeDefinition{
		// Transactions
		{
			method:         "GET",
			pattern:        "/transactions/{id}",
			name:           getTransactionByIDRoute,
			apiHandlerFunc: getTransactionByID,
		}, {
			method:         "POST",
			pattern:        "/transactions",
			name:           createTransactionRoute,
			apiHandlerFunc: createTransaction,
		},
		// Blocks
		{
			method:         "GET",
			pattern:        "/blocks/{id}",
			name:           getBlocksByIDRoute,
			apiHandlerFunc: getBlocksByID,
		}, {
			method:         "GET",
			pattern:        "/blocks",
			name:           getBlocksByHeightRoute,
			apiHandlerFunc: NotImplemented,
		},
		// Collections
		{
			method:         "GET",
			pattern:        "/collections/{id}",
			name:           getCollectionByIDRoute,
			apiHandlerFunc: NotImplemented,
		}, {}}
}
