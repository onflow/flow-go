// Package ingress implements accepting transactions into the system.
// It implements a subset of the Observation API.
package ingress

import (
	"context"
	"fmt"
	"net"

	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/collection/ingest"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/protobuf/services/observation"
)

// Config defines the configurable options for the ingress server.
type Config struct {
	ListenAddr string
}

// Ingress implements a gRPC server with a simplified version of the Observation
// API to enable receiving transactions into the system.
type Ingress struct {
	unit    *engine.Unit
	logger  zerolog.Logger
	handler *handler     // the gRPC service implementation
	server  *grpc.Server // the gRPC server
	config  Config
}

// New returns a new ingress server.
func New(config Config, e *ingest.Engine) *Ingress {
	ingress := &Ingress{
		unit: engine.NewUnit(),
		handler: &handler{
			engine: e,
		},
		server: grpc.NewServer(),
		config: config,
	}

	observation.RegisterObserveServiceServer(ingress.server, ingress.handler)

	return ingress
}

// Ready returns a ready channel that is closed once the module has fully
// started. The ingress module is ready when the gRPC server has successfully
// started.
func (i *Ingress) Ready() <-chan struct{} {
	i.unit.Launch(i.serve)
	return i.unit.Ready()
}

// Done returns a done channel that is closed once the module has fully stopped.
// It sends a signal to stop the gRPC server, then closes the channel.
func (i *Ingress) Done() <-chan struct{} {
	return i.unit.Done(i.server.GracefulStop)
}

// serve starts the gRPC server .
//
// When this function returns, the server is considered ready.
func (i *Ingress) serve() {

	log := i.logger.With().Str("module", "ingress").Logger()

	log.Info().Msgf("starting server on address %s", i.config.ListenAddr)

	l, err := net.Listen("tcp", i.config.ListenAddr)
	if err != nil {
		log.Err(err).Msg("failed to start server")
		return
	}

	err = i.server.Serve(l)
	if err != nil {
		log.Err(err).Msg("fatal error in server")
	}
}

// handler implements a subset of the Observation API.
type handler struct {
	engine network.Engine
}

var _ observation.ObserveServiceServer = &handler{}

// Ping responds to requests when the server is up.
func (h *handler) Ping(ctx context.Context, req *observation.PingRequest) (*observation.PingResponse, error) {
	return &observation.PingResponse{}, nil
}

// SendTransaction accepts new transactions and inputs them to the ingress
// engine for validation and routing.
func (h *handler) SendTransaction(ctx context.Context, req *observation.SendTransactionRequest) (*observation.SendTransactionResponse, error) {
	tx, err := MessageToTransaction(req.Transaction)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("failed to convert transaction: %v", err))
	}

	err = h.engine.ProcessLocal(&tx)
	if err != nil {
		return nil, err
	}

	txID := tx.ID()

	return &observation.SendTransactionResponse{Hash: txID[:]}, nil
}

// Remaining handler functions are no-ops to implement the Observation API
// protobuf service.
func (h *handler) GetLatestBlock(context.Context, *observation.GetLatestBlockRequest) (*observation.BlockResponse, error) {
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

func (h *handler) GetBlockByHash(context.Context, *observation.GetBlockByHashRequest) (*observation.BlockResponse, error) {
	return nil, nil
}

func (h *handler) GetBlockByHeight(context.Context, *observation.GetBlockByHeightRequest) (*observation.BlockResponse, error) {
	return nil, nil
}

func (h *handler) GetLatestBlockDetails(context.Context, *observation.GetLatestBlockDetailsRequest) (*observation.BlockDetailsResponse, error) {
	return nil, nil
}

func (h *handler) GetBlockDetailsByHash(context.Context, *observation.GetBlockDetailsByHashRequest) (*observation.BlockDetailsResponse, error) {
	return nil, nil
}

func (h *handler) GetBlockDetailsByHeight(context.Context, *observation.GetBlockDetailsByHeightRequest) (*observation.BlockDetailsResponse, error) {
	return nil, nil
}

func (h *handler) GetCollectionByHash(context.Context, *observation.GetCollectionByHashRequest) (*observation.CollectionResponse, error) {
	return nil, nil
}

func (h *handler) GetCollectionByHeight(context.Context, *observation.GetCollectionByHeightRequest) (*observation.CollectionResponse, error) {
	return nil, nil
}

func (h *handler) GetTransactionStatus(context.Context, *observation.GetTransactionRequest) (*observation.GetTransactionStatusResponse, error) {
	return nil, nil
}
