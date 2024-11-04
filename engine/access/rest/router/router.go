package router

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/http/middleware"
	"github.com/onflow/flow-go/engine/access/rest/http/models"
	"github.com/onflow/flow-go/engine/access/rest/http/routes"
	"github.com/onflow/flow-go/engine/access/rest/websockets/legacy"
	"github.com/onflow/flow-go/engine/access/state_stream"
	"github.com/onflow/flow-go/engine/access/state_stream/backend"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

// RouterBuilder is a utility for building HTTP routers with common middleware and routes.
type RouterBuilder struct {
	logger      zerolog.Logger
	router      *mux.Router
	v1SubRouter *mux.Router
}

// NewRouterBuilder creates a new RouterBuilder instance with common middleware and a v1 sub-router.
func NewRouterBuilder(
	logger zerolog.Logger,
	restCollector module.RestMetrics) *RouterBuilder {
	router := mux.NewRouter().StrictSlash(true)
	v1SubRouter := router.PathPrefix("/v1").Subrouter()

	// common middleware for all request
	v1SubRouter.Use(middleware.LoggingMiddleware(logger))
	v1SubRouter.Use(middleware.QueryExpandable())
	v1SubRouter.Use(middleware.QuerySelect())
	v1SubRouter.Use(middleware.MetricsMiddleware(restCollector))

	return &RouterBuilder{
		logger:      logger,
		router:      router,
		v1SubRouter: v1SubRouter,
	}
}

// AddRestRoutes adds rest routes to the router.
func (b *RouterBuilder) AddRestRoutes(backend access.API, chain flow.Chain) *RouterBuilder {
	linkGenerator := models.NewLinkGeneratorImpl(b.v1SubRouter)
	for _, r := range Routes {
		h := routes.NewHandler(b.logger, backend, r.Handler, linkGenerator, chain)
		b.v1SubRouter.
			Methods(r.Method).
			Path(r.Pattern).
			Name(r.Name).
			Handler(h)
	}
	return b
}

// AddWsRoutes adds WebSocket routes to the router.
func (b *RouterBuilder) AddWsRoutes(
	stateStreamApi state_stream.API,
	chain flow.Chain,
	stateStreamConfig backend.Config,
) *RouterBuilder {

	for _, r := range WSRoutes {
		h := legacy.NewWSHandler(b.logger, stateStreamApi, r.Handler, chain, stateStreamConfig)
		b.v1SubRouter.
			Methods(r.Method).
			Path(r.Pattern).
			Name(r.Name).
			Handler(h)
	}

	return b
}

func (b *RouterBuilder) Build() *mux.Router {
	return b.router
}

type route struct {
	Name    string
	Method  string
	Pattern string
	Handler routes.ApiHandlerFunc
}

type wsroute struct {
	Name    string
	Method  string
	Pattern string
	Handler legacy.SubscribeHandlerFunc
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
	Pattern: "/accounts/{address}/balance",
	Name:    "getAccountBalance",
	Handler: routes.GetAccountBalance,
}, {
	Method:  http.MethodGet,
	Pattern: "/accounts/{address}/keys/{index}",
	Name:    "getAccountKeyByIndex",
	Handler: routes.GetAccountKeyByIndex,
}, {
	Method:  http.MethodGet,
	Pattern: "/accounts/{address}/keys",
	Name:    "getAccountKeys",
	Handler: routes.GetAccountKeys,
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

var WSRoutes = []wsroute{{
	Method:  http.MethodGet,
	Pattern: "/subscribe_events",
	Name:    "subscribeEvents",
	Handler: legacy.SubscribeEvents,
}}

var routeUrlMap = map[string]string{}
var routeRE = regexp.MustCompile(`(?i)/v1/(\w+)(/(\w+))?(/(\w+))?(/(\w+))?`)

func init() {
	for _, r := range Routes {
		routeUrlMap[r.Pattern] = r.Name
	}
	for _, r := range WSRoutes {
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
	if len(matches) != 1 || len(matches[0]) != 8 {
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
		if matches[0][5] != "" {
			parts = append(parts, matches[0][5])
		}
	case 16:
		// address based resource. e.g. /v1/accounts/1234567890abcdef
		parts = append(parts, "{address}")
		if matches[0][5] == "keys" && matches[0][7] != "" {
			parts = append(parts, "keys", "{index}")
		} else if matches[0][5] != "" {
			parts = append(parts, matches[0][5])
		}
	default:
		// named resource. e.g. /v1/network/parameters
		parts = append(parts, matches[0][3])
	}

	return "/" + strings.Join(parts, "/"), nil
}
