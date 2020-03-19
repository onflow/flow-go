package rpc

import (
	"context"
	"net"

	"github.com/rs/zerolog"
	"google.golang.org/grpc"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/protobuf/services/observation"
)

// Config defines the configurable options for the gRPC server.
type Config struct {
	ListenAddr     string
	ExecutionAddr  string
	CollectionAddr string
}

// Engine implements a gRPC server with a simplified version of the Observation API.
type Engine struct {
	unit    *engine.Unit
	log     zerolog.Logger
	handler *Handler     // the gRPC service implementation
	server  *grpc.Server // the gRPC server
	config  Config
}

// New returns a new RPC engine.
func New(log zerolog.Logger, config Config, collectionRPC observation.ObserveServiceClient, executionRPC observation.ObserveServiceClient) *Engine {
	log = log.With().Str("engine", "rpc").Logger()

	eng := &Engine{
		log:  log,
		unit: engine.NewUnit(),
		handler: &Handler{
			collectionRPC: collectionRPC,
			executionRPC:  executionRPC,
		},
		server: grpc.NewServer(),
		config: config,
	}

	observation.RegisterObserveServiceServer(eng.server, eng.handler)

	return eng
}

// Ready returns a ready channel that is closed once the engine has fully
// started. The RPC engine is ready when the gRPC server has successfully
// started.
func (e *Engine) Ready() <-chan struct{} {
	e.unit.Launch(e.serve)
	return e.unit.Ready()
}

// Done returns a done channel that is closed once the engine has fully stopped.
// It sends a signal to stop the gRPC server, then closes the channel.
func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done(e.server.GracefulStop)
}

// serve starts the gRPC server .
//
// When this function returns, the server is considered ready.
func (e *Engine) serve() {
	e.log.Info().Msgf("starting server on address %s", e.config.ListenAddr)

	l, err := net.Listen("tcp", e.config.ListenAddr)
	if err != nil {
		e.log.Err(err).Msg("failed to start server")
		return
	}

	err = e.server.Serve(l)
	if err != nil {
		e.log.Err(err).Msg("fatal error in server")
	}
}

// Handler implements a subset of the Observation API
type Handler struct {
	executionRPC  observation.ObserveServiceClient
	collectionRPC observation.ObserveServiceClient
}

var _ observation.ObserveServiceServer = &Handler{}

// Ping responds to requests when the server is up.
func (h *Handler) Ping(ctx context.Context, req *observation.PingRequest) (*observation.PingResponse, error) {
	return &observation.PingResponse{}, nil
}

func (h *Handler) ExecuteScript(ctx context.Context, req *observation.ExecuteScriptRequest) (*observation.ExecuteScriptResponse, error) {
	return h.executionRPC.ExecuteScript(ctx, req)
}

// Remaining Handler functions are no-ops to implement the Observation API protobuf service.
func (h *Handler) SendTransaction(ctx context.Context, req *observation.SendTransactionRequest) (*observation.SendTransactionResponse, error) {
	return h.collectionRPC.SendTransaction(ctx, req)
}

func (h *Handler) GetLatestBlock(context.Context, *observation.GetLatestBlockRequest) (*observation.BlockResponse, error) {
	return nil, nil
}

func (h *Handler) GetTransaction(context.Context, *observation.GetTransactionRequest) (*observation.GetTransactionResponse, error) {
	return nil, nil
}

func (h *Handler) GetAccount(context.Context, *observation.GetAccountRequest) (*observation.GetAccountResponse, error) {
	return nil, nil
}

func (h *Handler) GetEvents(context.Context, *observation.GetEventsRequest) (*observation.GetEventsResponse, error) {
	return nil, nil
}

func (h *Handler) GetBlockByHash(context.Context, *observation.GetBlockByHashRequest) (*observation.BlockResponse, error) {
	return nil, nil
}

func (h *Handler) GetBlockByHeight(context.Context, *observation.GetBlockByHeightRequest) (*observation.BlockResponse, error) {
	return nil, nil
}

func (h *Handler) GetLatestBlockDetails(context.Context, *observation.GetLatestBlockDetailsRequest) (*observation.BlockDetailsResponse, error) {
	return nil, nil
}

func (h *Handler) GetBlockDetailsByHash(context.Context, *observation.GetBlockDetailsByHashRequest) (*observation.BlockDetailsResponse, error) {
	return nil, nil
}

func (h *Handler) GetBlockDetailsByHeight(context.Context, *observation.GetBlockDetailsByHeightRequest) (*observation.BlockDetailsResponse, error) {
	panic("implement me")
}

func (h *Handler) GetCollectionByHash(context.Context, *observation.GetCollectionByHashRequest) (*observation.CollectionResponse, error) {
	return nil, nil
}

func (h *Handler) GetCollectionByHeight(context.Context, *observation.GetCollectionByHeightRequest) (*observation.CollectionResponse, error) {
	return nil, nil
}

func (h *Handler) GetTransactionStatus(context.Context, *observation.GetTransactionRequest) (*observation.GetTransactionStatusResponse, error) {
	return nil, nil
}
