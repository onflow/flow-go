package rest

import (
	"github.com/onflow/flow-go/engine/access/rest/handlers"
	"github.com/onflow/flow-go/engine/access/rest/util"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/cors"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/middleware"
)

// NewServer returns an HTTP server initialized with the REST API handler
func NewServer(backend access.API, listenAddress string, logger zerolog.Logger) (*http.Server, error) {

	router, err := initRouter(backend, logger)
	if err != nil {
		return nil, err
	}

	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedHeaders: []string{"*"},
		AllowedMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodOptions,
			http.MethodHead},
	})

	return &http.Server{
		Addr:         listenAddress,
		Handler:      c.Handler(router),
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
	}, nil
}

func initRouter(backend access.API, logger zerolog.Logger) (*mux.Router, error) {
	router := mux.NewRouter().StrictSlash(true)
	v1SubRouter := router.PathPrefix("/v1").Subrouter()

	// common middleware for all request
	v1SubRouter.Use(middleware.LoggingMiddleware(logger))
	v1SubRouter.Use(middleware.QueryExpandable())
	v1SubRouter.Use(middleware.QuerySelect())

	var linkGenerator util.LinkGenerator = util.NewLinkGeneratorImpl(v1SubRouter)

	// create a schema validation
	validation, err := newSchemaValidation()
	if err != nil {
		return nil, err
	}

	for _, r := range handlers.Routes {
		h := handlers.NewHandler(logger, backend, r.Handler, linkGenerator, validation)
		v1SubRouter.
			Methods(r.Method).
			Path(r.Pattern).
			Name(r.Name).
			Handler(h)
	}
	return router, nil
}
