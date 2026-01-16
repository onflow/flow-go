package grpc

import (
	"context"
	"net"

	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/onflow/flow-go/ledger/remote/transport"
	ledgerpb "github.com/onflow/flow-go/ledger/protobuf"
)

// Server implements transport.ServerTransport using gRPC.
type Server struct {
	grpcServer *grpc.Server
	listener   net.Listener
	ready      chan struct{}
}

// NewServer creates a new gRPC transport server.
func NewServer(listener net.Listener, logger zerolog.Logger, maxRequestSize, maxResponseSize uint) *Server {
	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(int(maxRequestSize)),
		grpc.MaxSendMsgSize(int(maxResponseSize)),
	)

	return &Server{
		grpcServer: grpcServer,
		listener:   listener,
		ready:      make(chan struct{}),
	}
}

// Serve starts the transport server.
func (s *Server) Serve(handler transport.ServerHandler) error {
	// Adapt handler to gRPC service
	service := &serviceAdapter{handler: handler}
	ledgerpb.RegisterLedgerServiceServer(s.grpcServer, service)

	close(s.ready)
	return s.grpcServer.Serve(s.listener)
}

// Stop stops the transport server.
func (s *Server) Stop() {
	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}
}

// Ready returns a channel that is closed when the server is ready.
func (s *Server) Ready() <-chan struct{} {
	return s.ready
}

// serviceAdapter adapts transport.ServerHandler to gRPC LedgerServiceServer.
type serviceAdapter struct {
	ledgerpb.UnimplementedLedgerServiceServer
	handler transport.ServerHandler
}

func (s *serviceAdapter) InitialState(ctx context.Context, _ *emptypb.Empty) (*ledgerpb.StateResponse, error) {
	state, err := s.handler.InitialState(ctx)
	if err != nil {
		return nil, err
	}
	return &ledgerpb.StateResponse{State: state}, nil
}

func (s *serviceAdapter) HasState(ctx context.Context, req *ledgerpb.StateRequest) (*ledgerpb.HasStateResponse, error) {
	hasState, err := s.handler.HasState(ctx, req.State)
	if err != nil {
		return nil, err
	}
	return &ledgerpb.HasStateResponse{HasState: hasState}, nil
}

func (s *serviceAdapter) GetSingleValue(ctx context.Context, req *ledgerpb.GetSingleValueRequest) (*ledgerpb.ValueResponse, error) {
	value, err := s.handler.GetSingleValue(ctx, req.State, req.Key)
	if err != nil {
		return nil, err
	}
	return &ledgerpb.ValueResponse{Value: value}, nil
}

func (s *serviceAdapter) Get(ctx context.Context, req *ledgerpb.GetRequest) (*ledgerpb.GetResponse, error) {
	values, err := s.handler.Get(ctx, req.State, req.Keys)
	if err != nil {
		return nil, err
	}
	return &ledgerpb.GetResponse{Values: values}, nil
}

func (s *serviceAdapter) Set(ctx context.Context, req *ledgerpb.SetRequest) (*ledgerpb.SetResponse, error) {
	newState, trieUpdate, err := s.handler.Set(ctx, req.State, req.Keys, req.Values)
	if err != nil {
		return nil, err
	}
	return &ledgerpb.SetResponse{
		NewState:   newState,
		TrieUpdate: trieUpdate,
	}, nil
}

func (s *serviceAdapter) Prove(ctx context.Context, req *ledgerpb.ProveRequest) (*ledgerpb.ProofResponse, error) {
	proof, err := s.handler.Prove(ctx, req.State, req.Keys)
	if err != nil {
		return nil, err
	}
	return &ledgerpb.ProofResponse{Proof: proof}, nil
}
