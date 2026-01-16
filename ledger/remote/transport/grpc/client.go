package grpc

import (
	"context"

	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"

	ledgerpb "github.com/onflow/flow-go/ledger/protobuf"
)

// Client implements transport.ClientTransport using gRPC.
// This is a wrapper around the existing gRPC implementation.
type Client struct {
	conn   *grpc.ClientConn
	client ledgerpb.LedgerServiceClient
	logger zerolog.Logger
	ready  chan struct{}
}

// NewClient creates a new gRPC transport client.
func NewClient(addr string, logger zerolog.Logger, maxRequestSize, maxResponseSize uint) (*Client, error) {
	logger = logger.With().Str("component", "grpc_ledger_client").Logger()

	if maxRequestSize == 0 {
		maxRequestSize = 1 << 30 // 1 GiB
	}
	if maxResponseSize == 0 {
		maxResponseSize = 1 << 30 // 1 GiB
	}

	// Create gRPC connection
	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(int(maxResponseSize)),
			grpc.MaxCallSendMsgSize(int(maxRequestSize)),
		),
	)
	if err != nil {
		return nil, err
	}

	client := ledgerpb.NewLedgerServiceClient(conn)

	return &Client{
		conn:   conn,
		client: client,
		logger: logger,
		ready:  make(chan struct{}),
	}, nil
}

// InitialState returns the initial state of the ledger.
func (c *Client) InitialState(ctx context.Context) (*ledgerpb.State, error) {
	resp, err := c.client.InitialState(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, err
	}
	return resp.State, nil
}

// HasState checks if the given state exists in the ledger.
func (c *Client) HasState(ctx context.Context, state *ledgerpb.State) (bool, error) {
	req := &ledgerpb.StateRequest{State: state}
	resp, err := c.client.HasState(ctx, req)
	if err != nil {
		return false, err
	}
	return resp.HasState, nil
}

// GetSingleValue returns a single value for a given key at a specific state.
func (c *Client) GetSingleValue(ctx context.Context, state *ledgerpb.State, key *ledgerpb.Key) (*ledgerpb.Value, error) {
	req := &ledgerpb.GetSingleValueRequest{
		State: state,
		Key:   key,
	}
	resp, err := c.client.GetSingleValue(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.Value, nil
}

// Get returns values for multiple keys at a specific state.
func (c *Client) Get(ctx context.Context, state *ledgerpb.State, keys []*ledgerpb.Key) ([]*ledgerpb.Value, error) {
	req := &ledgerpb.GetRequest{
		State: state,
		Keys:  keys,
	}
	resp, err := c.client.Get(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.Values, nil
}

// Set updates keys with new values at a specific state and returns the new state.
func (c *Client) Set(ctx context.Context, state *ledgerpb.State, keys []*ledgerpb.Key, values []*ledgerpb.Value) (*ledgerpb.State, []byte, error) {
	req := &ledgerpb.SetRequest{
		State:  state,
		Keys:   keys,
		Values: values,
	}
	resp, err := c.client.Set(ctx, req)
	if err != nil {
		return nil, nil, err
	}
	return resp.NewState, resp.TrieUpdate, nil
}

// Prove returns proofs for the given keys at a specific state.
func (c *Client) Prove(ctx context.Context, state *ledgerpb.State, keys []*ledgerpb.Key) ([]byte, error) {
	req := &ledgerpb.ProveRequest{
		State: state,
		Keys:  keys,
	}
	resp, err := c.client.Prove(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.Proof, nil
}

// Close closes the transport connection.
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// Ready returns a channel that is closed when the transport is ready.
func (c *Client) Ready() <-chan struct{} {
	// For gRPC, we consider it ready immediately after connection
	// The actual readiness check is done in the client wrapper
	close(c.ready)
	return c.ready
}
