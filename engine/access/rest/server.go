package rest

import (
	"net/http"
	"time"

	"github.com/rs/cors"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/routes"
	"github.com/onflow/flow-go/engine/access/state_stream"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

const (
	// DefaultReadTimeout is the default read timeout for the HTTP server
	DefaultReadTimeout = time.Second * 15

	// DefaultWriteTimeout is the default write timeout for the HTTP server
	DefaultWriteTimeout = time.Second * 30

	// DefaultIdleTimeout is the default idle timeout for the HTTP server
	DefaultIdleTimeout = time.Second * 60
)

type Config struct {
	ListenAddress string
	WriteTimeout  time.Duration
	ReadTimeout   time.Duration
	IdleTimeout   time.Duration
}

// NewServer returns an HTTP server initialized with the REST API handler
func NewServer(serverAPI access.API,
	config Config,
	logger zerolog.Logger,
	chain flow.Chain,
	restCollector module.RestMetrics,
	stateStreamApi state_stream.API,
	eventFilterConfig state_stream.EventFilterConfig,
	maxGlobalStreams uint32,
) (*http.Server, error) {
	router, err := routes.NewRouter(serverAPI, logger, chain, restCollector, stateStreamApi, eventFilterConfig, maxGlobalStreams)
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
		Handler:      c.Handler(router),
		Addr:         config.ListenAddress,
		WriteTimeout: config.WriteTimeout,
		ReadTimeout:  config.ReadTimeout,
		IdleTimeout:  config.IdleTimeout,
	}, nil
}
