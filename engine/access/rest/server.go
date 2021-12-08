package rest

import (
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/middleware"
	"github.com/rs/cors"
	"github.com/rs/zerolog"
)

// all route names
const (
	getTransactionByIDRoute          = "getTransactionByID"
	getTransactionResultByIDRoute    = "getTransactionResultByID"
	createTransactionRoute           = "createTransaction"
	getBlocksByIDRoute               = "getBlocksByIDs"
	getBlocksByHeightRoute           = "getBlocksByHeight"
	getCollectionByIDRoute           = "getCollectionByID"
	executeScriptRoute               = "executeScript"
	getBlockPayloadByIDRoute         = "getBlockPayloadByID"
	getExecutionResultByBlockIDRoute = "getExecutionResultByBlockID"
	getExecutionResultByIDRoute      = "getExecutionResultByID"
	getAccountRoute                  = "getAccount"
)

// NewServer returns an HTTP server initialized with the REST API handler
func NewServer(backend access.API, listenAddress string, logger zerolog.Logger) *http.Server {

	router := initRouter(backend, logger)

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

func initRouter(backend access.API, logger zerolog.Logger) *mux.Router {
	router := mux.NewRouter().StrictSlash(true)
	v1SubRouter := router.PathPrefix("/v1").Subrouter()

	// common middleware for all request
	v1SubRouter.Use(middleware.LoggingMiddleware(logger))
	v1SubRouter.Use(middleware.QueryExpandable())
	v1SubRouter.Use(middleware.QuerySelect())

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
			method:         http.MethodGet,
			pattern:        "/transactions/{id}",
			name:           getTransactionByIDRoute,
			apiHandlerFunc: getTransactionByID,
		}, {
			method:         http.MethodPost,
			pattern:        "/transactions",
			name:           createTransactionRoute,
			apiHandlerFunc: createTransaction,
		},
		// Transaction Results
		{
			method:         http.MethodGet,
			pattern:        "/transaction_results/{id}",
			name:           getTransactionResultByIDRoute,
			apiHandlerFunc: getTransactionResultByID,
		},
		// Blocks
		{
			method:         http.MethodGet,
			pattern:        "/blocks/{id}",
			name:           getBlocksByIDRoute,
			apiHandlerFunc: getBlocksByIDs,
		}, {
			method:         http.MethodGet,
			pattern:        "/blocks",
			name:           getBlocksByHeightRoute,
			apiHandlerFunc: getBlocksByHeight,
		},
		// Block Payload
		{
			method:         http.MethodGet,
			pattern:        "/blocks/{id}/payload",
			name:           getBlockPayloadByIDRoute,
			apiHandlerFunc: getBlockPayloadByID,
		},
		// Execution Result
		{
			method:         http.MethodGet,
			pattern:        "/execution_results/{id}",
			name:           getExecutionResultByIDRoute,
			apiHandlerFunc: getExecutionResultByID,
		},
		{
			method:         http.MethodGet,
			pattern:        "/execution_results",
			name:           getExecutionResultByBlockIDRoute,
			apiHandlerFunc: getExecutionResultsByBlockIDs,
		},
		// Collections
		{
			method:         http.MethodGet,
			pattern:        "/collections/{id}",
			name:           getCollectionByIDRoute,
			apiHandlerFunc: getCollectionByID,
		},
		// Scripts
		{
			method:         http.MethodPost,
			pattern:        "/scripts",
			name:           executeScriptRoute,
			apiHandlerFunc: executeScript,
		},
		// Accounts
		{
			method:         http.MethodGet,
			pattern:        "/accounts/{address}",
			name:           getAccountRoute,
			apiHandlerFunc: getAccount,
		}}
}
