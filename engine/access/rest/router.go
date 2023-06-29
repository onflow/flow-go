package rest

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/access/rest/api"
	"github.com/onflow/flow-go/engine/access/rest/middleware"
	"github.com/onflow/flow-go/engine/access/rest/models"
	"github.com/onflow/flow-go/engine/access/rest/routes"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

func NewRouter(backend api.RestBackendApi, logger zerolog.Logger, chain flow.Chain, restCollector module.RestMetrics) (*mux.Router, error) {
	router := mux.NewRouter().StrictSlash(true)
	v1SubRouter := router.PathPrefix("/v1").Subrouter()

	// common middleware for all request
	v1SubRouter.Use(middleware.LoggingMiddleware(logger))
	v1SubRouter.Use(middleware.QueryExpandable())
	v1SubRouter.Use(middleware.QuerySelect())
	v1SubRouter.Use(middleware.MetricsMiddleware(restCollector))

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
	Handler: routes.GetTransactionByID,
}, {
	Method:  http.MethodPost,
	Pattern: "/transactions",
	Name:    "createTransaction",
	Handler: routes.CreateTransaction,
}, {
	Method:  http.MethodGet,
	Pattern: "/transaction_results/{id}",
	Name:    "getTransactionResultByID",
	Handler: routes.GetTransactionResultByID,
}, {
	Method:  http.MethodGet,
	Pattern: "/blocks/{id}",
	Name:    "getBlocksByIDs",
	Handler: routes.GetBlocksByIDs,
}, {
	Method:  http.MethodGet,
	Pattern: "/blocks",
	Name:    "getBlocksByHeight",
	Handler: routes.GetBlocksByHeight,
}, {
	Method:  http.MethodGet,
	Pattern: "/blocks/{id}/payload",
	Name:    "getBlockPayloadByID",
	Handler: routes.GetBlockPayloadByID,
}, {
	Method:  http.MethodGet,
	Pattern: "/execution_results/{id}",
	Name:    "getExecutionResultByID",
	Handler: routes.GetExecutionResultByID,
}, {
	Method:  http.MethodGet,
	Pattern: "/execution_results",
	Name:    "getExecutionResultByBlockID",
	Handler: routes.GetExecutionResultsByBlockIDs,
}, {
	Method:  http.MethodGet,
	Pattern: "/collections/{id}",
	Name:    "getCollectionByID",
	Handler: routes.GetCollectionByID,
}, {
	Method:  http.MethodPost,
	Pattern: "/scripts",
	Name:    "executeScript",
	Handler: routes.ExecuteScript,
}, {
	Method:  http.MethodGet,
	Pattern: "/accounts/{address}",
	Name:    "getAccount",
	Handler: routes.GetAccount,
}, {
	Method:  http.MethodGet,
	Pattern: "/events",
	Name:    "getEvents",
	Handler: routes.GetEvents,
}, {
	Method:  http.MethodGet,
	Pattern: "/network/parameters",
	Name:    "getNetworkParameters",
	Handler: routes.GetNetworkParameters,
}, {
	Method:  http.MethodGet,
	Pattern: "/node_version_info",
	Name:    "getNodeVersionInfo",
	Handler: routes.GetNodeVersionInfo,
}}
