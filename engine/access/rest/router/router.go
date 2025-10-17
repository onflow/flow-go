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
	"github.com/onflow/flow-go/engine/access/rest/common/models"
	flowhttp "github.com/onflow/flow-go/engine/access/rest/http"
	"github.com/onflow/flow-go/engine/access/rest/websockets"
	dp "github.com/onflow/flow-go/engine/access/rest/websockets/data_providers"
	legacyws "github.com/onflow/flow-go/engine/access/rest/websockets/legacy"
	"github.com/onflow/flow-go/engine/access/state_stream"
	"github.com/onflow/flow-go/engine/access/state_stream/backend"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/irrecoverable"
)

// RouterBuilder is a utility for building HTTP routers with common middleware and routes.
type RouterBuilder struct {
	logger      zerolog.Logger
	router      *mux.Router
	v1SubRouter *mux.Router

	LinkGenerator models.LinkGenerator
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
		logger:        logger,
		router:        router,
		v1SubRouter:   v1SubRouter,
		LinkGenerator: models.NewLinkGeneratorImpl(v1SubRouter),
	}
}

// AddRestRoutes adds rest routes to the router.
func (b *RouterBuilder) AddRestRoutes(
	backend access.API,
	chain flow.Chain,
	maxRequestSize int64,
	maxResponseSize int64,
) *RouterBuilder {
	for _, r := range Routes {
		h := flowhttp.NewHandler(b.logger, backend, r.Handler, b.LinkGenerator, chain, maxRequestSize, maxResponseSize)
		b.v1SubRouter.
			Methods(r.Method).
			Path(r.Pattern).
			Name(r.Name).
			Handler(h)
	}
	return b
}

// AddLegacyWebsocketsRoutes adds WebSocket routes to the router.
//
// Deprecated: Use AddWebsocketsRoute instead, which allows managing multiple streams with
// a single endpoint.
func (b *RouterBuilder) AddLegacyWebsocketsRoutes(
	stateStreamApi state_stream.API,
	chain flow.Chain,
	stateStreamConfig backend.Config,
	maxRequestSize int64,
	maxResponseSize int64,
) *RouterBuilder {

	for _, r := range WSLegacyRoutes {
		h := legacyws.NewWSHandler(b.logger, stateStreamApi, r.Handler, chain, stateStreamConfig, maxRequestSize, maxResponseSize)
		b.v1SubRouter.
			Methods(r.Method).
			Path(r.Pattern).
			Name(r.Name).
			Handler(h)
	}

	return b
}

func (b *RouterBuilder) AddWebsocketsRoute(
	ctx irrecoverable.SignalerContext,
	chain flow.Chain,
	config websockets.Config,
	maxRequestSize int64,
	maxResponseSize int64,
	dataProviderFactory dp.DataProviderFactory,
) *RouterBuilder {
	handler := websockets.NewWebSocketHandler(ctx, b.logger, config, chain, maxRequestSize, maxResponseSize, dataProviderFactory)
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

// the following logic is used to match the URL with the correct route metrics collection.
var routePatterns []*regexp.Regexp
var routeNameMap map[*regexp.Regexp]string

func init() {
	routePatterns = make([]*regexp.Regexp, 0, len(Routes)+len(WSLegacyRoutes))
	routeNameMap = make(map[*regexp.Regexp]string)

	// Convert REST route patterns to regex patterns for matching
	for _, r := range Routes {
		regexPattern := patternToRegex(r.Pattern)
		re := regexp.MustCompile("^" + regexPattern + "$")
		routePatterns = append(routePatterns, re)
		routeNameMap[re] = r.Name
	}

	// Convert WebSocket route patterns to regex patterns for matching
	for _, r := range WSLegacyRoutes {
		regexPattern := patternToRegex(r.Pattern)
		re := regexp.MustCompile("^" + regexPattern + "$")
		routePatterns = append(routePatterns, re)
		routeNameMap[re] = r.Name
	}
}

// patternToRegex converts a mux pattern like "/blocks/{id}" to a regex pattern
func patternToRegex(pattern string) string {
	// Escape special regex characters except for {}
	escaped := regexp.QuoteMeta(pattern)
	// Replace placeholder patterns with regex matchers
	// {id} -> matches 64 char hex string or integer
	escaped = strings.ReplaceAll(escaped, `\{id\}`, `([0-9a-fA-F]{64}|\d+)`)
	// {address} -> matches 16 char hex string
	escaped = strings.ReplaceAll(escaped, `\{address\}`, `[0-9a-fA-F]{16}`)
	// {index} -> matches integer
	escaped = strings.ReplaceAll(escaped, `\{index\}`, `\d+`)
	return escaped
}

// URLToRoute matches the URL against route patterns and returns the matching route name
func URLToRoute(url string) (string, error) {
	path := strings.TrimPrefix(url, "/v1")
	for _, pattern := range routePatterns {
		if pattern.MatchString(path) {
			return routeNameMap[pattern], nil
		}
	}
	return "", fmt.Errorf("no matching route found for URL: %s", url)
}
