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
	getTransactionByIDRoute       = "getTransactionByID"
	getTransactionResultByIDRoute = "getTransactionResultByID"
	createTransactionRoute        = "createTransaction"
	getBlocksByIDRoute            = "getBlocksByIDs"
	getBlocksByHeightRoute        = "getBlocksByHeight"
	getCollectionByIDRoute        = "getCollectionByID"
	executeScriptRoute            = "executeScript"
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
		h.addToRouter(v1SubRouter)
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
		// Transaction Results
		{
			method:         "GET",
			pattern:        "/transaction_results/{id}",
			name:           getTransactionResultByIDRoute,
			apiHandlerFunc: getTransactionResultByID,
		},
		// Blocks
		{
			method:         "GET",
			pattern:        "/blocks/{id}",
			name:           getBlocksByIDRoute,
			apiHandlerFunc: getBlocksByIDs,
		}, {
			method:         "GET",
			pattern:        "/blocks",
			name:           getBlocksByHeightRoute,
			apiHandlerFunc: getBlocksByHeights,
		},
		// Collections
		{
			method:         "GET",
			pattern:        "/collections/{id}",
			name:           getCollectionByIDRoute,
			apiHandlerFunc: getCollectionByID,
		},
		// Scripts
		{
			method:         "POST",
			pattern:        "/scripts",
			name:           executeScriptRoute,
			apiHandlerFunc: executeScript,
		}}
}
