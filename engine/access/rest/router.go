package rest

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/middleware"
	"github.com/onflow/flow-go/engine/access/rest/models"
	"github.com/onflow/flow-go/model/flow"
)

func newRouter(backend access.API, logger zerolog.Logger, chain flow.Chain) (*mux.Router, error) {
	router := mux.NewRouter().StrictSlash(true)
	v1SubRouter := router.PathPrefix("/v1").Subrouter()

	// common middleware for all request
	v1SubRouter.Use(middleware.LoggingMiddleware(logger))
	v1SubRouter.Use(middleware.QueryExpandable())
	v1SubRouter.Use(middleware.QuerySelect())

	linkGenerator := models.NewLinkGeneratorImpl(v1SubRouter)

	for _, r := range Routes {
		h := NewHandler(logger, backend, r.Handler, linkGenerator, chain)
		v1SubRouter.
			Methods(r.Method).
			Path(r.Pattern).
			Name(r.Name).
			Handler(h)
	}
	return router, nil
}

type route struct {
	Name    string
	Method  string
	Pattern string
	Handler ApiHandlerFunc
}

var Routes = []route{{
	Method:  http.MethodGet,
	Pattern: "/transactions/{id}",
	Name:    "getTransactionByID",
	Handler: GetTransactionByID,
}, {
	Method:  http.MethodPost,
	Pattern: "/transactions",
	Name:    "createTransaction",
	Handler: CreateTransaction,
}, {
	Method:  http.MethodGet,
	Pattern: "/transaction_results/{id}",
	Name:    "getTransactionResultByID",
	Handler: GetTransactionResultByID,
}, {
	Method:  http.MethodGet,
	Pattern: "/blocks/{id}",
	Name:    "getBlocksByIDs",
	Handler: GetBlocksByIDs,
}, {
	Method:  http.MethodGet,
	Pattern: "/blocks",
	Name:    "getBlocksByHeight",
	Handler: GetBlocksByHeight,
}, {
	Method:  http.MethodGet,
	Pattern: "/blocks/{id}/payload",
	Name:    "getBlockPayloadByID",
	Handler: GetBlockPayloadByID,
}, {
	Method:  http.MethodGet,
	Pattern: "/execution_results/{id}",
	Name:    "getExecutionResultByID",
	Handler: GetExecutionResultByID,
}, {
	Method:  http.MethodGet,
	Pattern: "/execution_results",
	Name:    "getExecutionResultByBlockID",
	Handler: GetExecutionResultsByBlockIDs,
}, {
	Method:  http.MethodGet,
	Pattern: "/collections/{id}",
	Name:    "getCollectionByID",
	Handler: GetCollectionByID,
}, {
	Method:  http.MethodPost,
	Pattern: "/scripts",
	Name:    "executeScript",
	Handler: ExecuteScript,
}, {
	Method:  http.MethodGet,
	Pattern: "/accounts/{address}",
	Name:    "getAccount",
	Handler: GetAccount,
}, {
	Method:  http.MethodGet,
	Pattern: "/events",
	Name:    "getEvents",
	Handler: GetEvents,
}}
