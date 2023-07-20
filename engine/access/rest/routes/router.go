package routes

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/middleware"
	"github.com/onflow/flow-go/engine/access/rest/models"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

func NewRouter(backend access.API, logger zerolog.Logger, chain flow.Chain, restCollector module.RestMetrics) (*mux.Router, error) {
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
}, {
	Method:  http.MethodGet,
	Pattern: "/network/parameters",
	Name:    "getNetworkParameters",
	Handler: GetNetworkParameters,
}, {
	Method:  http.MethodGet,
	Pattern: "/node_version_info",
	Name:    "getNodeVersionInfo",
	Handler: GetNodeVersionInfo,
}}

var routeUrlMap = map[string]string{}
var routeRE = regexp.MustCompile(`(?i)/v1/(\w+)(/(\w+)(/(\w+))?)?`)

func init() {
	for _, r := range Routes {
		routeUrlMap[r.Pattern] = r.Name
	}
}

func URLToRoute(url string) (string, error) {
	normalized, err := normalizeURL(url)
	if err != nil {
		return "", err
	}

	name, ok := routeUrlMap[normalized]
	if !ok {
		return "", fmt.Errorf("invalid url")
	}
	return name, nil
}

func normalizeURL(url string) (string, error) {
	matches := routeRE.FindAllStringSubmatch(url, -1)
	if len(matches) != 1 || len(matches[0]) != 6 {
		return "", fmt.Errorf("invalid url")
	}

	// given a URL like
	//      /v1/blocks/1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef/payload
	// groups  [  1  ] [                                3                             ] [  5  ]
	// normalized form like /v1/blocks/{id}/payload

	parts := []string{matches[0][1]}

	switch len(matches[0][3]) {
	case 0:
		// top level resource. e.g. /v1/blocks
	case 64:
		// id based resource. e.g. /v1/blocks/1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef
		parts = append(parts, "{id}")
	case 16:
		// address based resource. e.g. /v1/accounts/1234567890abcdef
		parts = append(parts, "{address}")
	default:
		// named resource. e.g. /v1/network/parameters
		parts = append(parts, matches[0][3])
	}

	if matches[0][5] != "" {
		parts = append(parts, matches[0][5])
	}

	return "/" + strings.Join(parts, "/"), nil
}
