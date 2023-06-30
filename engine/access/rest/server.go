package rest

import (
	"net/http"
	"time"

	"github.com/rs/cors"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/routes"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

// NewServer returns an HTTP server initialized with the REST API handler
func NewServer(serverAPI access.API, listenAddress string, logger zerolog.Logger, chain flow.Chain, restCollector module.RestMetrics) (*http.Server, error) {
	router, err := routes.NewRouter(serverAPI, logger, chain, restCollector)
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
