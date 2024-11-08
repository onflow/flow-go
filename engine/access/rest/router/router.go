package router

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/common/middleware"
	flowhttp "github.com/onflow/flow-go/engine/access/rest/http"
	"github.com/onflow/flow-go/engine/access/rest/http/models"
	"github.com/onflow/flow-go/engine/access/rest/websockets"
	legacyws "github.com/onflow/flow-go/engine/access/rest/websockets/legacy"
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
func (b *RouterBuilder) AddRestRoutes(
	backend access.API,
	chain flow.Chain,
	maxRequestSize int64,
) *RouterBuilder {
	linkGenerator := models.NewLinkGeneratorImpl(b.v1SubRouter)
	for _, r := range Routes {
		h := flowhttp.NewHandler(b.logger, backend, r.Handler, linkGenerator, chain, maxRequestSize)
		b.v1SubRouter.
			Methods(r.Method).
			Path(r.Pattern).
			Name(r.Name).
			Handler(h)
	}
	return b
}

// AddLegacyWebsocketsRoutes adds WebSocket routes to the router.
func (b *RouterBuilder) AddLegacyWebsocketsRoutes(
	stateStreamApi state_stream.API,
	chain flow.Chain,
	stateStreamConfig backend.Config,
	maxRequestSize int64,
) *RouterBuilder {

	for _, r := range WSLegacyRoutes {
		h := legacyws.NewWSHandler(b.logger, stateStreamApi, r.Handler, chain, stateStreamConfig, maxRequestSize)
		b.v1SubRouter.
			Methods(r.Method).
			Path(r.Pattern).
			Name(r.Name).
			Handler(h)
	}

	return b
}

func (b *RouterBuilder) AddWebsocketsRoute(
	chain flow.Chain,
	config *websockets.Config,
	streamApi state_stream.API,
	streamConfig backend.Config,
	maxRequestSize int64,
) *RouterBuilder {
	handler := websockets.NewWebSocketHandler(b.logger, config, chain, streamApi, streamConfig, maxRequestSize)
	b.v1SubRouter.
		Methods(http.MethodGet).
		Path("/ws").
		Name("ws").
		Handler(handler)

	return b
}

func (b *RouterBuilder) Build() *mux.Router {
	return b.router
}

var routeUrlMap = map[string]string{}
var routeRE = regexp.MustCompile(`(?i)/v1/(\w+)(/(\w+))?(/(\w+))?(/(\w+))?`)

func init() {
	for _, r := range Routes {
		routeUrlMap[r.Pattern] = r.Name
	}
	for _, r := range WSLegacyRoutes {
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
