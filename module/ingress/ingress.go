// Package ingress implements accepting transactions into the system.
// It implements a subset of the Observation API.
package ingress

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/collection/ingest"
	"github.com/dapperlabs/flow-go/protobuf/services/observation"
	"github.com/dapperlabs/flow-go/sdk/convert"
)

// Config defines the configurable options for the ingress server.
type Config struct {
	ListenAddr string
}

// Ingress implements a GRPC server with a simplified version of the Observation
// API to enable receiving transactions into the system.
type Ingress struct {
	logger  zerolog.Logger
	handler *handler     // the GRPC service implementation
	server  *grpc.Server // the GRPC server
	config  Config

	stop    chan struct{}  // indicates that the server should stop
	stopped sync.WaitGroup // indicates that all goroutines have stopped
}

// New returns a new ingress server.
func New(config Config, engine *ingest.Engine) *Ingress {
	ingress := &Ingress{
		handler: &handler{
			engine: engine,
		},
		server: grpc.NewServer(),
		config: config,
		stop:   make(chan struct{}),
	}

	observation.RegisterObserveServiceServer(ingress.server, ingress.handler)

	return ingress
}

// Ready returns a ready channel that is closed once the module has fully
// started. The ingress module is ready when the GRPC server has successfully
// started.
func (i *Ingress) Ready() <-chan struct{} {
	ready := make(chan struct{})

	go func() {
		i.start()
		close(ready)
	}()

	return ready
}

// Done returns a done channel that is closed once the module has fully stopped.
// It sends a signal to stop the GRPC server, then closes the channel.
func (i Ingress) Done() <-chan struct{} {
	done := make(chan struct{})

	go func() {
		<-i.stop
		i.stopped.Wait()
		close(done)
	}()

	return done
}

// start starts the GRPC server in a goroutine.
//
// When this function returns, the server is considered ready.
// This function starts a goroutine to handle stop signals.
func (i *Ingress) start() {
	log := i.logger.With().Str("module", "ingress").Logger()

	log.Info().Msgf("starting server on address %s", i.config.ListenAddr)

	l, err := net.Listen("tcp", i.config.ListenAddr)
	if err != nil {
		log.Err(err).Msg("failed to start server")
		return
	}

	// wait for a stop signal, then gracefully stop the server
	go func() {
		select {
		case <-i.stop:
			i.server.GracefulStop()
		}
	}()

	// asynchronously start the server
	go func() {
		i.stopped.Add(1)
		defer i.stopped.Done()

		err = i.server.Serve(l)
		if err != nil {
			log.Err(err).Msg("fatal error in server")
		}
	}()
}

// handler implements a subset of the Observation API.
type handler struct {
	engine engine.Engine
}

// Ping responds to requests when the server is up.
func (h *handler) Ping(ctx context.Context, req *observation.PingRequest) (*observation.PingResponse, error) {
	return &observation.PingResponse{}, nil
}

// SendTransaction accepts new transactions and inputs them to the ingress
// engine for validation and routing.
func (h *handler) SendTransaction(ctx context.Context, req *observation.SendTransactionRequest) (*observation.SendTransactionResponse, error) {
	tx, err := convert.MessageToTransaction(req.Transaction)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("failed to convert transaction: %v", err))
	}

	err = h.engine.Submit(&tx)
	if err != nil {
		return nil, err
	}

	return &observation.SendTransactionResponse{Hash: tx.Hash()}, nil
}

// Remaining handler functions are no-ops to implement the Observation API
// protobuf service.
func (h *handler) GetLatestBlock(context.Context, *observation.GetLatestBlockRequest) (*observation.GetLatestBlockResponse, error) {
	return nil, nil
}

func (h *handler) GetTransaction(context.Context, *observation.GetTransactionRequest) (*observation.GetTransactionResponse, error) {
	return nil, nil
}

func (h *handler) GetAccount(context.Context, *observation.GetAccountRequest) (*observation.GetAccountResponse, error) {
	return nil, nil
}

func (h *handler) ExecuteScript(context.Context, *observation.ExecuteScriptRequest) (*observation.ExecuteScriptResponse, error) {
	return nil, nil
}

func (h *handler) GetEvents(context.Context, *observation.GetEventsRequest) (*observation.GetEventsResponse, error) {
	return nil, nil
}
